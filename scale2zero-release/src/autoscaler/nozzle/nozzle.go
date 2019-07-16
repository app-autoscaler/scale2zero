package nozzle

import (
	"context"
	"crypto/tls"
	"time"

	"autoscaler/appmanager"

	loggregator "code.cloudfoundry.org/go-loggregator"
	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"code.cloudfoundry.org/lager"
	// "github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
	// "autoscaler/healthendpoint"
)

// const METRICS_FORWARDER_ORIGIN = "autoscaler_metrics_forwarder"

var selectors = []*loggregator_v2.Selector{
	{
		Message: &loggregator_v2.Selector_Gauge{
			Gauge: &loggregator_v2.GaugeSelector{},
		},
	},
	{
		Message: &loggregator_v2.Selector_Timer{
			Timer: &loggregator_v2.TimerSelector{},
		},
	},
}

// var envelopeCounter = prometheus.CounterOpts{
// 	Namespace: "autoscaler",
// 	Subsystem: "metricsgateway",
// 	Name:      "envelope_number_from_rlp",
// 	Help:      "the total envelopes number got from rlp",
// }

type EnvelopeStreamerLogger struct {
	logger lager.Logger
}

func (l *EnvelopeStreamerLogger) Printf(message string, data ...interface{}) {
	l.logger.Debug(message, lager.Data{"data": data})
}
func (l *EnvelopeStreamerLogger) Panicf(message string, data ...interface{}) {
	l.logger.Fatal(message, nil, lager.Data{"data": data})
}

type Nozzle struct {
	logger        lager.Logger
	rlpAddr       string
	tls           *tls.Config
	envelopChan   chan *loggregator_v2.Envelope
	index         int
	shardID       string
	appIDs        map[string]string
	getAppIDsFunc appmanager.GetAppsFunc
	context       context.Context
	cancelFunc    context.CancelFunc
}

func NewNozzle(logger lager.Logger, index int, shardID string, rlpAddr string, tls *tls.Config,
	envelopChan chan *loggregator_v2.Envelope, getAppIDsFunc appmanager.GetAppsFunc) *Nozzle {
	ctx, cancelFunc := context.WithCancel(context.Background())
	return &Nozzle{
		logger:        logger.Session("Nozzle"),
		index:         index,
		shardID:       shardID,
		rlpAddr:       rlpAddr,
		tls:           tls,
		envelopChan:   envelopChan,
		getAppIDsFunc: getAppIDsFunc,
		context:       ctx,
		cancelFunc:    cancelFunc,
	}
}

func (n *Nozzle) Start() {
	go n.streamMetrics()
	n.logger.Info("started", lager.Data{"index": n.index})
}

func (n *Nozzle) Stop() {
	n.cancelFunc()
}

func (n *Nozzle) streamMetrics() {
	streamConnector := loggregator.NewEnvelopeStreamConnector(n.rlpAddr, n.tls,
		loggregator.WithEnvelopeStreamLogger(&EnvelopeStreamerLogger{
			logger: n.logger.Session("envelope_streamer"),
		}),
		loggregator.WithEnvelopeStreamConnectorDialOptions(grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                10 * time.Second,
			Timeout:             30 * time.Second,
			PermitWithoutStream: true,
		})),
	)
	rx := streamConnector.Stream(n.context, &loggregator_v2.EgressBatchRequest{
		ShardId:   n.shardID,
		Selectors: selectors,
	})
	for {
		select {
		case <-n.context.Done():
			n.logger.Info("nozzle-stopped", lager.Data{"index": n.index})
			return
		default:
		}
		envelops := rx()
		if envelops != nil {
			n.filterEnvelopes(envelops)
		}

	}
}

func (n *Nozzle) filterEnvelopes(envelops []*loggregator_v2.Envelope) {
	for _, e := range envelops {
		_, exist := n.getAppIDsFunc()[e.SourceId]
		if exist {
			switch e.GetMessage().(type) {
			case *loggregator_v2.Envelope_Timer:
				if e.GetTimer().GetName() == "http" {
					n.logger.Debug("filter-envelopes-get-httpstartstop", lager.Data{"index": n.index, "appID": e.SourceId, "message": e.Message})
					n.envelopChan <- e
				}
			}
		}
	}
}