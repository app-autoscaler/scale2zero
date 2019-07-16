package appmanager

import (
	"context"
	"sync"
	"time"

	"autoscaler/db"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
)

type GetAppsFunc func() map[string]int
type AddAppFunc func(string, int)
type RemoveAppFunc func(string)

type AppManager struct {
	logger          lager.Logger
	refreshInterval time.Duration
	nodeNum         int
	nodeIndex       int
	applicationDB   db.ApplicationDB
	clock           clock.Clock
	applicationMap  map[string]int
	pLock           sync.RWMutex
	mLock           sync.RWMutex
	context         context.Context
	stopFunc        context.CancelFunc
}

func NewAppManager(logger lager.Logger, clock clock.Clock, refreshInterval time.Duration,
	applicationDB db.ApplicationDB) *AppManager {
	ctx, cancelFunc := context.WithCancel(context.Background())
	return &AppManager{
		logger:          logger.Session("AppManager"),
		clock:           clock,
		refreshInterval: refreshInterval,
		applicationDB:   applicationDB,
		applicationMap:  make(map[string]int),
		context:         ctx,
		stopFunc:        cancelFunc,
	}
}
func (am *AppManager) GetApplications() map[string]int {
	am.pLock.RLock()
	defer am.pLock.RUnlock()
	return am.applicationMap
}
func (am *AppManager) Start() {
	go am.startApplicationRetrieve()
	am.logger.Info("started", lager.Data{"refreshInterval": am.refreshInterval})
}

func (am *AppManager) Stop() {
	am.stopFunc()
	am.logger.Info("stopped")
}

func (am *AppManager) startApplicationRetrieve() {
	tick := am.clock.NewTicker(am.refreshInterval)
	defer tick.Stop()

	for {
		am.logger.Debug("getting applications from db")
		apps, err := am.applicationDB.GetApplications()
		if err == nil {
			am.pLock.Lock()
			am.applicationMap = apps
			am.pLock.Unlock()
		}

		select {
		case <-am.context.Done():
			return
		case <-tick.C():
		}
	}
}
func (am *AppManager) AddApp(appId string, breachDuration int) {
	am.pLock.Lock()
	am.logger.Debug("add app", lager.Data{"appId": appId, "breachDuration": breachDuration})
	am.applicationMap[appId] = breachDuration
	am.pLock.Unlock()
}
func (am *AppManager) RemoveApp(appId string) {
	am.pLock.Lock()
	am.logger.Debug("remove app", lager.Data{"appId": appId})
	delete(am.applicationMap, appId)
	am.pLock.Unlock()
}
