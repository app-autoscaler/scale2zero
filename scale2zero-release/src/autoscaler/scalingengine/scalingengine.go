package scalingengine

import (
	"errors"
	"fmt"
	"time"

	"autoscaler/cf"
	"autoscaler/db"
	"autoscaler/models"
	"autoscaler/routemanager"

	"code.cloudfoundry.org/lager"
)

type Scale2ZeroFunc func(*models.Trigger) error
type StartAppFunc func(string) error
type ScalingEngine struct {
	logger                   lager.Logger
	cfClient                 cf.Client
	scalingEngineDB          db.ScalingEngineDB
	appNextScaleTimestampMap map[string]int64
	disableAppRoutesFunc     routemanager.DisableAppRoutesFunc
	enableAppRoutesFunc      routemanager.EnableAppRoutesFunc
	coolDownDuration         time.Duration
	appLock                  *StripedLock
	startStopExpiredDuration time.Duration
}

func NewScalingEngine(logger lager.Logger, cfClient cf.Client, scalingEngineDB db.ScalingEngineDB, enableAppRoutesFunc routemanager.EnableAppRoutesFunc, disableAppRoutesFunc routemanager.DisableAppRoutesFunc, coolDownDuration time.Duration, lockSize int) *ScalingEngine {
	return &ScalingEngine{
		logger:                   logger.Session("scalingengine"),
		cfClient:                 cfClient,
		scalingEngineDB:          scalingEngineDB,
		appNextScaleTimestampMap: map[string]int64{},
		disableAppRoutesFunc:     disableAppRoutesFunc,
		enableAppRoutesFunc:      enableAppRoutesFunc,
		coolDownDuration:         coolDownDuration,
		appLock:                  NewStripedLock(lockSize),
		startStopExpiredDuration: 2 * time.Minute,
	}
}

func (se *ScalingEngine) StartApp(appID string) error {
	se.appLock.GetLock(appID).Lock()
	defer se.appLock.GetLock(appID).Unlock()
	se.logger.Info("start application", lager.Data{"appID": appID})
	history := &models.AppScalingHistory{
		AppId:       appID,
		Timestamp:   time.Now().UnixNano(),
		ScalingType: models.ScalingType2ZeroStart,
		Reason:      "ingress request received",
	}

	defer se.scalingEngineDB.SaveScalingHistory(history)
	err := se.cfClient.StartApp(appID)
	if err != nil {
		se.logger.Error("failed to start application", err, lager.Data{"appID": appID})
		return err
	}
	expiredChan := time.After(se.startStopExpiredDuration)
	for {
		select {
		case <-expiredChan:
			se.logger.Info("expired but there is no running instances", lager.Data{"appID": appID})
			history.Status = models.ScalingStatusFailed
			history.Message = fmt.Sprintf("failed to start application in %d seconds", se.startStopExpiredDuration/time.Second)
			return errors.New("failed to start application")
		default:
			num, err := se.cfClient.GetAppRunningInstanceNumber(appID)
			if err != nil {
				se.logger.Error("failed to get app running instance number", err, lager.Data{"appID": appID})
			}
			if num >= 1 {
				se.logger.Debug("start finished,app running instance number >= 1", lager.Data{"appID": appID, "running_num": num})
				se.appNextScaleTimestampMap[appID] = time.Now().Add(se.coolDownDuration).UnixNano()
				se.disableAppRoutesFunc(appID)
				history.Status = models.ScalingStatusSucceeded
				return nil
			}
			se.logger.Debug("app running instance number is 0", lager.Data{"appID": appID})
			time.Sleep(5 * time.Second)

		}
	}
}

func (se *ScalingEngine) StopApp(trigger *models.Trigger) error {
	appID := trigger.AppID
	se.appLock.GetLock(appID).Lock()
	defer se.appLock.GetLock(appID).Unlock()
	se.logger.Info("stop application", lager.Data{"trigger": trigger})
	if !se.checkCoolDown(appID) {
		se.logger.Debug("can not stop app because it is in cooldown duration", lager.Data{"trigger": trigger})
		return nil
	}
	history := &models.AppScalingHistory{
		AppId:       appID,
		Timestamp:   time.Now().UnixNano(),
		ScalingType: models.ScalingType2ZeroStop,
		Reason:      fmt.Sprintf("no ingress request received for %d seconds", trigger.BreachDuration),
	}

	// defer se.scalingEngineDB.SaveScalingHistory(history)
	appSummary, err := se.cfClient.GetAppSummary(appID)
	if err != nil {
		se.logger.Error("failed to get app summary when stopping application", err, lager.Data{"appId": appID})
		history.Status = models.ScalingStatusFailed
		history.Message = "failed to get application status"
		se.scalingEngineDB.SaveScalingHistory(history)
		return err
	}
	if appSummary.State == "STOPPED" {
		se.logger.Info("app alreay stopped", lager.Data{"appID": appID})
		return nil
	}
	err = se.cfClient.StopApp(appID)
	if err != nil {
		se.logger.Error("failed to stop application", err, lager.Data{"appID": appID})
		history.Status = models.ScalingStatusFailed
		history.Message = "failed to send stop application request"
		se.scalingEngineDB.SaveScalingHistory(history)
		return err
	}
	expiredChan := time.After(se.startStopExpiredDuration)
	for {
		select {
		case <-expiredChan:
			se.logger.Info("expired but there are still running instances", lager.Data{"appID": appID})
			history.Status = models.ScalingStatusFailed
			history.Message = fmt.Sprintf("failed to stop application in %d seconds", se.startStopExpiredDuration/time.Second)
			se.scalingEngineDB.SaveScalingHistory(history)
			return errors.New("failed to stop app")
		default:
			appSummary, err := se.cfClient.GetAppSummary(appID)
			if err != nil {
				se.logger.Error("check app summary,failed to get app summary", err, lager.Data{"appId": appID})
				history.Status = models.ScalingStatusFailed
				history.Message = "failed to get application status"
				se.scalingEngineDB.SaveScalingHistory(history)
				return err
			}
			if appSummary.State == "STOPPED" {
				se.logger.Info("check app summary,app alreay stopped", lager.Data{"appID": appID})
				se.appNextScaleTimestampMap[appID] = time.Now().Add(se.coolDownDuration).UnixNano()
				se.enableAppRoutesFunc(appID)
				history.Status = models.ScalingStatusSucceeded
				se.scalingEngineDB.SaveScalingHistory(history)
				return nil
			}
			se.logger.Debug("app has not stopped", lager.Data{"appID": appID})
			time.Sleep(5 * time.Second)

		}
	}
}

func (se *ScalingEngine) checkCoolDown(appID string) bool {
	duration := time.Now().UnixNano() - se.appNextScaleTimestampMap[appID]
	se.logger.Debug("check cooldown", lager.Data{"duration(s)": duration / 1000000000})
	if duration > 0 {
		return true
	}
	return false
}
