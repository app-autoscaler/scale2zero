package routemanager

import (
	"context"
	"time"

	"autoscaler/cf"
	"autoscaler/models"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
)

type RouteEmitter struct {
	logger        lager.Logger
	client        cf.Client
	flushInterval time.Duration
	routeChan     chan models.RouteEntity
	flushClock    clock.Clock
	context       context.Context
	stopFunc      context.CancelFunc
}

func NewRouteEmitter(logger lager.Logger, client cf.Client, flushInterval time.Duration, routeChan chan models.RouteEntity,
	flushClock clock.Clock) *RouteEmitter {
	ctx, cancelFunc := context.WithCancel(context.Background())
	return &RouteEmitter{
		logger:        logger.Session("route_emitter"),
		client:        client,
		flushInterval: flushInterval,
		routeChan:     routeChan,
		flushClock:    flushClock,
		context:       ctx,
		stopFunc:      cancelFunc,
	}
}

func (re *RouteEmitter) Start() {
	go re.startToEmitRoute()
	re.logger.Info("started")
}
func (re *RouteEmitter) Stop() {
	re.stopFunc()
}
func (re *RouteEmitter) startToEmitRoute() {
	ticker := re.flushClock.NewTicker(re.flushInterval)
	var routeList []models.RouteEntity = make([]models.RouteEntity, 0, 1000)
	for {
		select {
		case <-re.context.Done():
			re.logger.Info("stopped")
			return
		case <-ticker.C():
			re.Emit(routeList)
			routeList = make([]models.RouteEntity, 0, 1000)
		default:
			routeList = append(routeList, <-re.routeChan)
		}
	}
}
func (re *RouteEmitter) Emit(routes []models.RouteEntity) {
	re.logger.Debug("emit routes", lager.Data{"routes": routes})
	err := re.client.RegisterRoutes(routes)
	re.logger.Debug("finished emit routes", lager.Data{"routes": routes, "err": err})
	if err != nil {
		re.logger.Error("failed to register routes", err, lager.Data{"routes": routes})
	}
}
