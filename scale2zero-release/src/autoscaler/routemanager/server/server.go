package server

import (
	"fmt"
	"net/http"

	"autoscaler/routemanager"
	"autoscaler/scalingengine"

	"code.cloudfoundry.org/lager"
	"github.com/gorilla/mux"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/http_server"
)

type VarsFunc func(w http.ResponseWriter, r *http.Request, vars map[string]string)

func (vh VarsFunc) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	vh(w, r, vars)
}

func NewServer(logger lager.Logger, serverPort int, startAppFunc scalingengine.StartAppFunc,
	getAppIDByRouteFunc routemanager.GetAppIDByRouteFunc) (ifrit.Runner, error) {
	rh := NewRouteHandler(logger, startAppFunc, getAppIDByRouteFunc)
	router := mux.NewRouter()
	router.PathPrefix("/").Handler(VarsFunc(rh.HijackAppRoutes))

	addr := fmt.Sprintf("0.0.0.0:%d", serverPort)

	var runner ifrit.Runner
	runner = http_server.New(addr, router)

	logger.Info("http-server-created", lager.Data{"port": serverPort})
	return runner, nil
}
