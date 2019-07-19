package server

import (
	"net/http"
	"net/http/httputil"

	"autoscaler/models"
	"autoscaler/routemanager"
	"autoscaler/scalingengine"

	"code.cloudfoundry.org/cfhttp/handlers"
	"code.cloudfoundry.org/lager"
)

type RouteHandler struct {
	logger              lager.Logger
	getAppIDByRouteFunc routemanager.GetAppIDByRouteFunc
	startAppFunc        scalingengine.StartAppFunc
	reverseProxy        *httputil.ReverseProxy
}

func NewRouteHandler(logger lager.Logger, startAppFunc scalingengine.StartAppFunc, getAppIDByRouteFunc routemanager.GetAppIDByRouteFunc) *RouteHandler {
	return &RouteHandler{
		logger:              logger.Session("route_handler"),
		getAppIDByRouteFunc: getAppIDByRouteFunc,
		startAppFunc:        startAppFunc,
		reverseProxy: &httputil.ReverseProxy{
			Director: func(req *http.Request) {
				req.URL.Host = req.Host
				req.URL.Scheme = "http"
				if _, ok := req.Header["User-Agent"]; !ok {
					// explicitly disable User-Agent so it's not set to default value
					req.Header.Set("User-Agent", "")
				}
			},
		},
	}
}

func (h *RouteHandler) HijackAppRoutes(w http.ResponseWriter, r *http.Request, vars map[string]string) {
	host := r.Host
	appID := h.getAppIDByRouteFunc(host)
	h.logger.Debug("request comming", lager.Data{"appId": appID, "host": host})
	if appID == "" {
		h.logger.Info("there is no app found for route", lager.Data{"route": host})
		handlers.WriteJSONResponse(w, http.StatusInternalServerError, models.ErrorResponse{
			Code:    "Internal-server-error",
			Message: "Error taking scaling action"})
		return
	}
	err := h.startAppFunc(appID)
	if err != nil {
		h.logger.Error("failed to start application", err, lager.Data{"appID": appID})
		handlers.WriteJSONResponse(w, http.StatusInternalServerError, models.ErrorResponse{
			Code:    "Internal-server-error",
			Message: "Error taking scaling action"})
		return
	}
	// scheme := "https"
	// if r.Proto == "HTTP/1.1" {
	// 	scheme = "http"
	// }
	// targetUrl, _ := url.Parse(scheme + "://" + host)
	// httputil.NewSingleHostReverseProxy(targetUrl).ServeHTTP(w, r)
	h.reverseProxy.ServeHTTP(w, r)
}
