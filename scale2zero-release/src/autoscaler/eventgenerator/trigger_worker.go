package eventgenerator

import (
	"context"

	"autoscaler/models"
	"autoscaler/scalingengine"

	"code.cloudfoundry.org/lager"
)

type TriggerWorker struct {
	logger         lager.Logger
	triggerChan    chan *models.Trigger
	scale2ZeroFunc scalingengine.Scale2ZeroFunc
	context        context.Context
	stopFunc       context.CancelFunc
}

func NewTriggerWorker(logger lager.Logger, triggerChan chan *models.Trigger, scale2ZeroFunc scalingengine.Scale2ZeroFunc) *TriggerWorker {
	ctx, cancelFunc := context.WithCancel(context.Background())
	return &TriggerWorker{
		logger:         logger.Session("TriggerWorker"),
		triggerChan:    triggerChan,
		scale2ZeroFunc: scale2ZeroFunc,
		context:        ctx,
		stopFunc:       cancelFunc,
	}
}

func (w *TriggerWorker) Start() {
	go w.doHandleTrigger()
	w.logger.Info("started")
}
func (w *TriggerWorker) Stop() {
	w.stopFunc()
}

func (w *TriggerWorker) doHandleTrigger() {
	for {
		select {
		case <-w.context.Done():
			w.logger.Info("stopped")
		case trigger := <-w.triggerChan:
			w.logger.Debug("scale2zero", lager.Data{"trigger": trigger})
			err := w.scale2ZeroFunc(trigger)
			if err != nil {
				w.logger.Error("failed to stop application", err, lager.Data{"trigger": trigger})
			}

		}
	}
}
