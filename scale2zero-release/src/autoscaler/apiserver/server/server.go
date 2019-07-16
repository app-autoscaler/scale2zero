package server

import (
	"fmt"
	"net/http"

	"autoscaler/appmanager"
	"autoscaler/cf"
	"autoscaler/db"
	"autoscaler/routes"

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

func NewServer(logger lager.Logger, serverPort int, applicationDB db.ApplicationDB, scalingEngineDB db.ScalingEngineDB, cfClient cf.Client,
	addAppFunc appmanager.AddAppFunc, removeAppFunc appmanager.RemoveAppFunc) (ifrit.Runner, error) {
	rh := NewPublicApiHandler(logger, applicationDB, scalingEngineDB, addAppFunc, removeAppFunc)
	oAuthMiddleware := NewOauthMiddleware(logger, cfClient)
	r := routes.PublicApiRoutes()
	r.Use(oAuthMiddleware.CheckPermission)
	r.Get(routes.EnableScale2ZeroRouteName).Handler(VarsFunc(rh.Enable))
	r.Get(routes.DisableScale2ZeroRouteName).Handler(VarsFunc(rh.Disable))
	r.Get(routes.ScalingHistoryRouteName).Handler(VarsFunc(rh.GetScalingHistories))
	addr := fmt.Sprintf("0.0.0.0:%d", serverPort)

	var runner ifrit.Runner
	runner = http_server.New(addr, r)

	logger.Info("http-server-created", lager.Data{"port": serverPort})
	return runner, nil
}
