package models

type Policy struct {
	BreachDuration int `json:"breach_duration"`
}

type Trigger struct {
	AppID          string `json:"app_id"`
	BreachDuration int    `json:"breach_duration"`
}
