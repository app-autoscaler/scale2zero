package server

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"

	"autoscaler/appmanager"
	"autoscaler/db"
	"autoscaler/models"

	"code.cloudfoundry.org/cfhttp/handlers"
	"code.cloudfoundry.org/lager"
)

type PublicApiHandler struct {
	logger        lager.Logger
	removeAppFunc appmanager.RemoveAppFunc
	addAppFunc    appmanager.AddAppFunc
	// scalingEngine *scalingengine.ScalingEngine
	applicationDB   db.ApplicationDB
	scalingEngineDB db.ScalingEngineDB
}

func NewPublicApiHandler(logger lager.Logger, applicationDB db.ApplicationDB, scalingEngineDB db.ScalingEngineDB, addAppFunc appmanager.AddAppFunc, removeAppFunc appmanager.RemoveAppFunc) *PublicApiHandler {
	return &PublicApiHandler{
		logger:          logger.Session("publicapiserver"),
		applicationDB:   applicationDB,
		scalingEngineDB: scalingEngineDB,
		addAppFunc:      addAppFunc,
		removeAppFunc:   removeAppFunc,
		// scalingEngine: scalingEngine,
	}
}

func (h *PublicApiHandler) Enable(w http.ResponseWriter, r *http.Request, vars map[string]string) {
	appId := vars["appid"]
	if appId == "" {
		h.logger.Error("AppId is missing", nil, nil)
		handlers.WriteJSONResponse(w, http.StatusBadRequest, models.ErrorResponse{
			Code:    "Bad Request",
			Message: "AppId is required",
		})
		return
	}

	h.logger.Info("enable scale2zero", lager.Data{"appId": appId})
	policyBytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		h.logger.Error("failed to read request body", err, lager.Data{"appId": appId})
		handlers.WriteJSONResponse(w, http.StatusInternalServerError, models.ErrorResponse{
			Code:    "Interal-Server-Error",
			Message: "Failed to read request body"})
		return
	}
	var policy models.Policy
	err = json.Unmarshal(policyBytes, &policy)
	if err != nil {
		h.logger.Error("failed to unmarshal request body", err, lager.Data{"appId": appId, "body": string(policyBytes)})
		handlers.WriteJSONResponse(w, http.StatusInternalServerError, models.ErrorResponse{
			Code:    "Interal-Server-Error",
			Message: "Failed to unmarshal request body"})
		return
	}
	err = h.applicationDB.SaveApplication(appId, policy.BreachDuration)
	if err != nil {
		h.logger.Error("failed to save appliaction", err, lager.Data{"appId": appId})
		handlers.WriteJSONResponse(w, http.StatusInternalServerError, models.ErrorResponse{
			Code:    "Internal-Server-Error",
			Message: "Error saving application",
		})
		return
	}
	h.addAppFunc(appId, policy.BreachDuration)
	handlers.WriteJSONResponse(w, http.StatusOK, nil)

}

func (h *PublicApiHandler) Disable(w http.ResponseWriter, r *http.Request, vars map[string]string) {
	appId := vars["appid"]
	if appId == "" {
		h.logger.Error("AppId is missing", nil, nil)
		handlers.WriteJSONResponse(w, http.StatusBadRequest, models.ErrorResponse{
			Code:    "Bad Request",
			Message: "AppId is required",
		})
		return
	}

	h.logger.Info("enable scale2zero", lager.Data{"appId": appId})
	err := h.applicationDB.DeleteApplication(appId)
	if err != nil {
		h.logger.Error("failed to delete appliaction", err, lager.Data{"appId": appId})
		handlers.WriteJSONResponse(w, http.StatusInternalServerError, models.ErrorResponse{
			Code:    "Internal-Server-Error",
			Message: "Error deleting application",
		})
		return
	}
	h.removeAppFunc(appId)
	handlers.WriteJSONResponse(w, http.StatusOK, nil)

}

func (h *PublicApiHandler) GetScalingHistories(w http.ResponseWriter, r *http.Request, vars map[string]string) {

	appId := vars["appid"]
	logger := h.logger.Session("get-scaling-histories", lager.Data{"appId": appId})

	startParam := r.URL.Query()["start"]
	endParam := r.URL.Query()["end"]
	orderParam := r.URL.Query()["order"]
	includeParam := r.URL.Query()["include"]
	logger.Debug("handling", lager.Data{"start": startParam, "end": endParam})

	var err error
	start := int64(0)
	end := int64(-1)
	order := db.DESC
	includeAll := false

	if len(startParam) == 1 {
		start, err = strconv.ParseInt(startParam[0], 10, 64)
		if err != nil {
			logger.Error("failed-to-parse-start-time", err, lager.Data{"start": startParam})
			handlers.WriteJSONResponse(w, http.StatusBadRequest, models.ErrorResponse{
				Code:    "Bad-Request",
				Message: "Error parsing start time"})
			return
		}
	} else if len(startParam) > 1 {
		logger.Error("failed-to-get-start-time", err, lager.Data{"start": startParam})
		handlers.WriteJSONResponse(w, http.StatusBadRequest, models.ErrorResponse{
			Code:    "Bad-Request",
			Message: "Incorrect start parameter in query string"})
		return
	}

	if len(endParam) == 1 {
		end, err = strconv.ParseInt(endParam[0], 10, 64)
		if err != nil {
			logger.Error("failed-to-parse-end-time", err, lager.Data{"end": endParam})
			handlers.WriteJSONResponse(w, http.StatusBadRequest, models.ErrorResponse{
				Code:    "Bad-Request",
				Message: "Error parsing end time"})
			return
		}
	} else if len(endParam) > 1 {
		logger.Error("failed-to-get-end-time", err, lager.Data{"end": endParam})
		handlers.WriteJSONResponse(w, http.StatusBadRequest, models.ErrorResponse{
			Code:    "Bad-Request",
			Message: "Incorrect end parameter in query string"})
		return
	}

	if len(orderParam) == 1 {
		orderStr := strings.ToUpper(orderParam[0])
		if orderStr == db.DESCSTR {
			order = db.DESC
		} else if orderStr == db.ASCSTR {
			order = db.ASC
		} else {
			logger.Error("failed-to-get-order", err, lager.Data{"order": orderParam})
			handlers.WriteJSONResponse(w, http.StatusBadRequest, models.ErrorResponse{
				Code:    "Bad-Request",
				Message: fmt.Sprintf("Incorrect order parameter in query string, the value can only be '%s' or '%s'", db.ASCSTR, db.DESCSTR),
			})
			return
		}
	} else if len(orderParam) > 1 {
		logger.Error("failed-to-get-order", err, lager.Data{"order": orderParam})
		handlers.WriteJSONResponse(w, http.StatusBadRequest, models.ErrorResponse{
			Code:    "Bad-Request",
			Message: "Incorrect order parameter in query string"})
		return
	}

	if len(includeParam) == 1 {
		includeStr := strings.ToLower(includeParam[0])
		if includeStr == "all" {
			includeAll = true
		} else {
			logger.Error("failed-to-get-include-parameter", err, lager.Data{"include": includeParam})
			handlers.WriteJSONResponse(w, http.StatusBadRequest, models.ErrorResponse{
				Code:    "Bad-Request",
				Message: fmt.Sprintf("Incorrect include parameter in query string, the value can only be 'all'"),
			})
			return
		}
	} else if len(includeParam) > 1 {
		logger.Error("failed-to-get-include-parameter", err, lager.Data{"include": includeParam})
		handlers.WriteJSONResponse(w, http.StatusBadRequest, models.ErrorResponse{
			Code:    "Bad-Request",
			Message: "Incorrect include parameter in query string"})
		return
	}

	var histories []*models.AppScalingHistory

	histories, err = h.scalingEngineDB.RetrieveScalingHistories(appId, start, end, order, includeAll)
	if err != nil {
		logger.Error("failed-to-retrieve-histories", err, lager.Data{"start": start, "end": end, "order": order, "includeAll": includeAll})
		handlers.WriteJSONResponse(w, http.StatusInternalServerError, models.ErrorResponse{
			Code:    "Interal-Server-Error",
			Message: "Error getting scaling histories from database"})
		return
	}

	var body []byte
	body, err = json.Marshal(histories)
	if err != nil {
		logger.Error("failed-to-marshal", err, lager.Data{"histories": histories})

		handlers.WriteJSONResponse(w, http.StatusInternalServerError, models.ErrorResponse{
			Code:    "Interal-Server-Error",
			Message: "Error getting scaling histories from database"})
		return
	}

	w.Write(body)
}
