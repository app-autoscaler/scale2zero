package routes

import (
	"github.com/gorilla/mux"

	"net/http"
)

const (
	Scale2ZeroPath             = "/v1/apps/{appid}/scale2zero"
	EnableScale2ZeroRouteName  = "EnableScale2Zero"
	DisableScale2ZeroRouteName = "DisableScale2Zero"

	ScalingHistoryPath      = "/v1/apps/{appid}/scaling_histories"
	ScalingHistoryRouteName = "GetScalingHistories"
)

type AutoScalerRoute struct {
	scalingEngineRoutes *mux.Router
	publicApiRoutes     *mux.Router
	routeHijackRoutes   *mux.Router
}

var autoScalerRouteInstance = newRouters()

func newRouters() *AutoScalerRoute {
	instance := &AutoScalerRoute{
		publicApiRoutes:   mux.NewRouter(),
		routeHijackRoutes: mux.NewRouter(),
	}

	instance.publicApiRoutes.Path(ScalingHistoryPath).Methods(http.MethodGet).Name(ScalingHistoryRouteName)

	instance.publicApiRoutes.Path(Scale2ZeroPath).Methods(http.MethodPut).Name(EnableScale2ZeroRouteName)
	instance.publicApiRoutes.Path(Scale2ZeroPath).Methods(http.MethodDelete).Name(DisableScale2ZeroRouteName)

	return instance

}

func PublicApiRoutes() *mux.Router {
	return autoScalerRouteInstance.publicApiRoutes
}
