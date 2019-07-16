package routemanager

import (
	"context"
	"sync"
	"time"

	"autoscaler/appmanager"
	"autoscaler/cf"
	"autoscaler/models"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
)

type GetAppIDByRouteFunc func(string) string
type DisableAppRoutesFunc func(string)
type EnableAppRoutesFunc func(string)

type RouteManager struct {
	logger                  lager.Logger
	clock                   clock.Clock
	sendRouteInterval       time.Duration
	refreshAppInterval      time.Duration
	cfClient                cf.Client
	appRoutesMap            map[string][]string
	routeAppMap             map[string]string
	temporaryDisabledAppMap map[string]bool
	routeListenerIP         string
	routeListenerPort       int
	getAppFunc              appmanager.GetAppsFunc
	routeChan               chan models.RouteEntity
	appRoutesMapLock        sync.RWMutex
	context                 context.Context
	stopFunc                context.CancelFunc
}

func NewRouteManager(logger lager.Logger, clock clock.Clock, sendRouteInterval time.Duration,
	refreshAppInterval time.Duration, cfClient cf.Client, routeListenerIP string, routeListenerPort int,
	getAppFunc appmanager.GetAppsFunc, routeChan chan models.RouteEntity) *RouteManager {
	ctx, cancelFunc := context.WithCancel(context.Background())
	return &RouteManager{
		logger:                  logger.Session("route_manager"),
		clock:                   clock,
		sendRouteInterval:       sendRouteInterval,
		refreshAppInterval:      refreshAppInterval,
		cfClient:                cfClient,
		appRoutesMap:            map[string][]string{},
		routeAppMap:             map[string]string{},
		temporaryDisabledAppMap: map[string]bool{},
		routeListenerIP:         routeListenerIP,
		routeListenerPort:       routeListenerPort,
		getAppFunc:              getAppFunc,
		routeChan:               routeChan,
		appRoutesMapLock:        sync.RWMutex{},
		context:                 ctx,
		stopFunc:                cancelFunc,
	}
}

func (rm *RouteManager) Start() {
	go rm.startRefreshApp()
	go rm.startSendRoute()
	rm.logger.Info("started")
}
func (rm *RouteManager) Stop() {
	rm.stopFunc()
	rm.logger.Info("stopped")
}

func (rm *RouteManager) startRefreshApp() {
	refreshAppTicker := rm.clock.NewTicker(rm.refreshAppInterval)
	for {
		select {
		case <-rm.context.Done():
			rm.logger.Info("stop refreshing apps")
			return
		case <-refreshAppTicker.C():
			rm.logger.Debug("refreshing apps")
			rm.refreshApps()
		}
	}
}
func (rm *RouteManager) startSendRoute() {
	sendRouteTicker := rm.clock.NewTicker(rm.sendRouteInterval)
	for {
		select {
		case <-rm.context.Done():
			rm.logger.Info("stop sending routes")
			return
		case <-sendRouteTicker.C():
			rm.logger.Debug("sending routes")
			rm.sendRoutes()
		}
	}
}
func (rm *RouteManager) refreshApps() {
	appMap := rm.getAppFunc()
	newAppList := []string{}
	staleAppList := []string{}
	for appId, _ := range appMap {
		if _, exists := rm.appRoutesMap[appId]; !exists {
			newAppList = append(newAppList, appId)
		}
	}
	for appId, _ := range rm.appRoutesMap {
		if _, exists := appMap[appId]; !exists {
			staleAppList = append(staleAppList, appId)
		}
	}
	rm.appRoutesMapLock.Lock()
	for _, appId := range staleAppList {
		if routes, exists := rm.appRoutesMap[appId]; exists {
			for _, r := range routes {
				delete(rm.routeAppMap, r)
			}
		}
		delete(rm.appRoutesMap, appId)
	}
	for _, appId := range newAppList {
		routes, err := rm.cfClient.GetAppRoutes(appId)
		if err != nil {
			rm.logger.Error("failed to get app routes", err, lager.Data{"appId": appId})
			continue
		}
		for _, r := range routes {
			rm.routeAppMap[r] = appId
		}
		rm.appRoutesMap[appId] = routes
	}
	rm.appRoutesMapLock.Unlock()

}
func (rm *RouteManager) sendRoutes() {
	rm.logger.Debug("app_routes_map length", lager.Data{"len": len(rm.appRoutesMap)})
	for appID, routes := range rm.appRoutesMap {
		if rm.temporaryDisabledAppMap[appID] {
			rm.logger.Debug("Stop to report route for app is started", lager.Data{"appID": appID})
			continue
		}
		rm.logger.Debug("app_routes", lager.Data{"appId": appID, "routes": routes})
		for _, route := range routes {
			rm.logger.Debug("emit routes", lager.Data{"appId": appID, "route": route})
			rm.routeChan <- models.RouteEntity{
				Route: route,
				IP:    rm.routeListenerIP,
				Port:  rm.routeListenerPort,
				TTL:   120,
			}
		}

	}
}

func (rm *RouteManager) DisableAppRoutes(appID string) {
	rm.appRoutesMapLock.Lock()
	rm.temporaryDisabledAppMap[appID] = true
	rm.appRoutesMapLock.Unlock()
	if routes, exists := rm.appRoutesMap[appID]; exists {
		routeEntityList := []models.RouteEntity{}
		for _, r := range routes {
			routeEntityList = append(routeEntityList, models.RouteEntity{
				Route: r,
				IP:    rm.routeListenerIP,
				Port:  rm.routeListenerPort,
				TTL:   120,
			})
		}
		rm.logger.Debug("unregister app routes", lager.Data{"appID": appID, "routes": routeEntityList})
		if len(routeEntityList) > 0 {
			rm.cfClient.UnRegisterRoutes(routeEntityList)

		}

	}

}

func (rm *RouteManager) EnableAppRoutes(appID string) {
	rm.appRoutesMapLock.Lock()
	delete(rm.temporaryDisabledAppMap, appID)
	rm.appRoutesMapLock.Unlock()
}

func (rm *RouteManager) GetAppIDByRoute(route string) string {
	rm.appRoutesMapLock.RLock()
	defer rm.appRoutesMapLock.RUnlock()
	if appID, exists := rm.routeAppMap[route]; exists {
		return appID
	}
	return ""

}
