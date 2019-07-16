package main

import (
	apiServer "autoscaler/apiserver/server"
	"autoscaler/appmanager"
	"autoscaler/cf"
	"autoscaler/config"
	"autoscaler/db"
	"autoscaler/db/sqldb"
	"autoscaler/eventgenerator"
	"autoscaler/helpers"
	"autoscaler/models"
	"autoscaler/nozzle"
	"autoscaler/routemanager"
	routeListenerServer "autoscaler/routemanager/server"
	"autoscaler/scalingengine"

	"crypto/tls"
	"flag"
	"fmt"
	"io/ioutil"
	"os"

	"code.cloudfoundry.org/clock"
	loggregator "code.cloudfoundry.org/go-loggregator"
	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"code.cloudfoundry.org/lager"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/sigmon"
)

func main() {
	var path string
	flag.StringVar(&path, "c", "", "config file")
	flag.Parse()
	if path == "" {
		fmt.Fprintln(os.Stdout, "missing config file\nUsage:use '-c' option to specify the config file path")
		os.Exit(1)
	}
	conf, err := loadConfig(path)
	if err != nil {
		fmt.Fprintf(os.Stdout, "%s\n", err.Error())
		os.Exit(1)
	}
	logger := helpers.InitLoggerFromConfig(&conf.Logging, "scale2zero")
	clock := clock.NewClock()

	applicationDB, err := sqldb.NewApplicationSQLDB(conf.DB.ApplicationDB, logger.Session("application-db"))
	if err != nil {
		logger.Error("failed to connect application database", err, lager.Data{"dbConfig": conf.DB.ApplicationDB})
		os.Exit(1)
	}
	defer applicationDB.Close()

	var scalingEngineDB db.ScalingEngineDB
	scalingEngineDB, err = sqldb.NewScalingEngineSQLDB(conf.DB.ScalingEngineDB, logger.Session("scalingengine-db"))
	if err != nil {
		logger.Error("failed to connect scalingengine database", err, lager.Data{"dbConfig": conf.DB.ScalingEngineDB})
		os.Exit(1)
	}
	defer scalingEngineDB.Close()
	appManager := appmanager.NewAppManager(logger, clock, conf.AppManager.AppRefreshInterval, applicationDB)

	cfClient, err := cf.NewClient(&conf.CF, logger)
	if err != nil {
		logger.Error("failed to create cf client", err, lager.Data{"cfConfig": conf.CF})
		os.Exit(1)
	}
	routeChan := make(chan models.RouteEntity, conf.RouteChanSize)

	routeEmitter := routemanager.NewRouteEmitter(logger, cfClient, conf.RouteEmitter.FlushInterval, routeChan, clock)
	routeManager := routemanager.NewRouteManager(logger, clock, conf.RouteManager.SendRouteInterval, conf.RouteManager.RefreshAppRouteInterval,
		cfClient, conf.RouteListener.IPAddress, conf.RouteListener.Port, appManager.GetApplications, routeChan)
	scalingEngine := scalingengine.NewScalingEngine(logger, cfClient, scalingEngineDB, routeManager.EnableAppRoutes, routeManager.DisableAppRoutes, conf.CoolDownDuration, conf.ScalingEngine.LockSize)

	routeListenerServer, err := routeListenerServer.NewServer(logger.Session("route_listener"), conf.RouteListener.Port, scalingEngine.StartApp, routeManager.GetAppIDByRoute)
	if err != nil {
		logger.Error("failed to create routelistener http server", err)
		os.Exit(1)
	}
	loggregatorClientTLSConfig, err := loggregator.NewEgressTLSConfig(conf.Nozzle.RLPClientTLS.CACertFile, conf.Nozzle.RLPClientTLS.CertFile, conf.Nozzle.RLPClientTLS.KeyFile)
	if err != nil {
		fmt.Fprintf(os.Stdout, "%s\n", err.Error())
		os.Exit(1)
	}
	evelopeChan := make(chan *loggregator_v2.Envelope, conf.EnvelopeChanSize)
	triggerChan := make(chan *models.Trigger, conf.TriggerChanSize)
	aggregator := eventgenerator.NewAggregator(logger, evelopeChan, triggerChan, clock, conf.Aggregator.EvaluationInterval, conf.Aggregator.AppRefreshInterval, appManager.GetApplications, conf.CoolDownDuration)
	triggerWorkers := createTriggerWorkers(conf.TriggerWorkerCount, logger, triggerChan, scalingEngine.StopApp)
	nozzles := createNozzles(conf.NozzleCount, logger, conf.Nozzle.ShardID, conf.Nozzle.RLPAddr, loggregatorClientTLSConfig, evelopeChan, appManager.GetApplications)
	apiServer, err := apiServer.NewServer(logger, conf.ApiServer.Server.Port, applicationDB, scalingEngineDB, cfClient, appManager.AddApp, appManager.RemoveApp)
	if err != nil {
		logger.Error("failed to create api http server", err)
		os.Exit(1)
	}
	scale2Zeroer := ifrit.RunFunc(func(signals <-chan os.Signal, ready chan<- struct{}) error {
		appManager.Start()
		routeEmitter.Start()
		routeManager.Start()
		aggregator.Start()

		for _, worker := range triggerWorkers {
			worker.Start()
		}
		for _, nozzle := range nozzles {
			nozzle.Start()
		}

		close(ready)

		<-signals
		for _, nozzle := range nozzles {
			nozzle.Stop()
		}
		for _, worker := range triggerWorkers {
			worker.Stop()
		}

		aggregator.Start()
		routeManager.Start()
		routeEmitter.Start()
		appManager.Start()

		return nil
	})
	members := grouper.Members{
		{"scale2Zeroer", scale2Zeroer},
		{"apiServer", apiServer},
		{"routeListenerServer", routeListenerServer},
	}

	monitor := ifrit.Invoke(sigmon.New(grouper.NewOrdered(os.Interrupt, members)))

	logger.Info("started")

	err = <-monitor.Wait()
	if err != nil {
		logger.Error("exited-with-failure", err)
		os.Exit(1)
	}

	logger.Info("exited")
}
func loadConfig(path string) (*config.Config, error) {
	configFile, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open config file %q: %s", path, err.Error())
	}

	configFileBytes, err := ioutil.ReadAll(configFile)
	configFile.Close()
	if err != nil {
		return nil, fmt.Errorf("failed to read data from config file %q: %s", path, err.Error())
	}

	conf, err := config.LoadConfig(configFileBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config file %q: %s", path, err.Error())
	}

	err = conf.Validate()
	if err != nil {
		return nil, fmt.Errorf("failed to validate configuration: %s", err.Error())
	}
	return conf, nil
}

func createTriggerWorkers(workerCount int, logger lager.Logger, triggerChan chan *models.Trigger, scale2ZeroFunc scalingengine.Scale2ZeroFunc) []*eventgenerator.TriggerWorker {
	triggerWorkers := make([]*eventgenerator.TriggerWorker, workerCount)
	for i := 0; i < workerCount; i++ {
		triggerWorkers[i] = eventgenerator.NewTriggerWorker(logger, triggerChan, scale2ZeroFunc)
	}
	return triggerWorkers
}

func createNozzles(nozzleCount int, logger lager.Logger, shardID string, rlpAddr string, loggregatorClientTLSConfig *tls.Config, envelopChan chan *loggregator_v2.Envelope, getAppIDsFunc appmanager.GetAppsFunc) []*nozzle.Nozzle {

	nozzles := make([]*nozzle.Nozzle, nozzleCount)
	for i := 0; i < nozzleCount; i++ {
		nozzles[i] = nozzle.NewNozzle(logger, i, shardID, rlpAddr, loggregatorClientTLSConfig, envelopChan, getAppIDsFunc)
	}
	return nozzles
}
