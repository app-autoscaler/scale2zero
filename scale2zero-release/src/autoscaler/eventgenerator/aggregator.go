package eventgenerator

import (
	"context"
	"time"

	"autoscaler/appmanager"
	"autoscaler/models"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"code.cloudfoundry.org/lager"
)

type Aggregator struct {
	logger                  lager.Logger
	envelopChan             chan *loggregator_v2.Envelope
	triggerChan             chan *models.Trigger
	appLatestRequestTimeMap map[string]int64
	appBreachDurationMap    map[string]int
	clock                   clock.Clock
	evaluationInterval      time.Duration
	appRefreshInterval      time.Duration
	getAppsFunc             appmanager.GetAppsFunc
	context                 context.Context
	stopFunc                context.CancelFunc
	coolDownDuration        time.Duration
}

func NewAggregator(logger lager.Logger, envelopChan chan *loggregator_v2.Envelope, triggerChan chan *models.Trigger, clock clock.Clock,
	evaluationInterval time.Duration, appRefreshInterval time.Duration, getAppsFunc appmanager.GetAppsFunc, coolDownDuration time.Duration) *Aggregator {
	ctx, cancelFunc := context.WithCancel(context.Background())
	return &Aggregator{
		logger:                  logger.Session("eventgenerator"),
		envelopChan:             envelopChan,
		triggerChan:             triggerChan,
		appLatestRequestTimeMap: map[string]int64{},
		appBreachDurationMap:    map[string]int{},
		clock:                   clock,
		context:                 ctx,
		stopFunc:                cancelFunc,
		evaluationInterval:      evaluationInterval,
		appRefreshInterval:      appRefreshInterval,
		getAppsFunc:             getAppsFunc,
		coolDownDuration:        coolDownDuration,
	}
}

func (eg *Aggregator) Start() {
	go eg.startAggregation()
	go eg.startEvaluation()
	go eg.startRefreshApp()
	eg.logger.Info("started")
}
func (eg *Aggregator) Stop() {
	eg.stopFunc()
	eg.logger.Info("stopped")
}
func (eg *Aggregator) startAggregation() {
	for {
		select {
		case <-eg.context.Done():
			eg.logger.Info("aggregator stopped")
			return
		case envelope := <-eg.envelopChan:
			eg.logger.Debug("app request", lager.Data{"appId": envelope.SourceId})
			eg.appLatestRequestTimeMap[envelope.SourceId] = time.Now().UnixNano()

		}
	}
}

func (eg *Aggregator) startEvaluation() {
	evaluationTicker := eg.clock.NewTicker(eg.evaluationInterval)
	for {
		select {
		case <-eg.context.Done():
			eg.logger.Info("evaluator stopped")
			return
		case <-evaluationTicker.C():
			eg.doEvaluation()

		}
	}
}
func (eg *Aggregator) startRefreshApp() {
	refreshAppTicker := eg.clock.NewTicker(eg.appRefreshInterval)
	for {
		select {
		case <-eg.context.Done():
			eg.logger.Info("evaluator stopped")
			return
		case <-refreshAppTicker.C():
			eg.refreshApps()

		}
	}
}

func (eg *Aggregator) doEvaluation() {
	for appID, lastRequestTime := range eg.appLatestRequestTimeMap {
		eg.logger.Debug("do evaluation", lager.Data{"appId": appID})
		duration := (time.Now().UnixNano() - lastRequestTime) / 1000000000
		eg.logger.Debug("duration for no http request", lager.Data{"appID": appID, "duration(s)": duration})
		if duration > int64(eg.appBreachDurationMap[appID]) {
			eg.logger.Debug("send trigger", lager.Data{"appId": appID, "breachDuration": int64(eg.appBreachDurationMap[appID])})
			eg.triggerChan <- &models.Trigger{
				AppID:          appID,
				BreachDuration: eg.appBreachDurationMap[appID],
			}
			// eg.appLatestRequestTimeMap[appID] = time.Now().Add(eg.coolDownDuration).UnixNano()
		}
	}
}

func (eg *Aggregator) refreshApps() {
	apps := eg.getAppsFunc()
	eg.appBreachDurationMap = apps
	for appId, _ := range apps {
		if _, exists := eg.appLatestRequestTimeMap[appId]; !exists {
			eg.appLatestRequestTimeMap[appId] = time.Now().UnixNano()
		}
	}
	for appId, _ := range eg.appLatestRequestTimeMap {
		if _, exists := apps[appId]; !exists {
			delete(eg.appLatestRequestTimeMap, appId)
		}
	}
}
