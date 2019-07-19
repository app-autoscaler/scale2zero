package models

type AppEntity struct {
	Instances int     `json:"instances"`
	State     *string `json:"state,omitempty"`
}

type ScalingType int
type ScalingStatus int

const (
	ScalingTypeDynamic ScalingType = iota
	ScalingTypeSchedule
	ScalingType2ZeroStop
	ScalingType2ZeroStart
)

const (
	ScalingStatusSucceeded ScalingStatus = iota
	ScalingStatusFailed
	ScalingStatusIgnored
)

const (
	AppStatusStopped = "STOPPED"
	AppStatusStarted = "STARTED"
)

type AppScalingHistory struct {
	AppId        string        `json:"app_id"`
	Timestamp    int64         `json:"timestamp"`
	ScalingType  ScalingType   `json:"scaling_type"`
	Status       ScalingStatus `json:"status"`
	OldInstances int           `json:"old_instances"`
	NewInstances int           `json:"new_instances"`
	Reason       string        `json:"reason"`
	Message      string        `json:"message"`
	Error        string        `json:"error"`
}

type AppScalingResult struct {
	AppId             string        `json:"app_id"`
	Status            ScalingStatus `json:"status"`
	Adjustment        int           `json:"adjustment"`
	CooldownExpiredAt int64         `json:"cool_down_expired_at"`
}
