package models

//RouteEntity is the object to emit to routing api
type RouteEntity struct {
	Route string `json:"route"`
	IP    string `json:"ip"`
	Port  int    `json:"port"`
	TTL   int    `json:"ttl"`
}
type Route struct {
	AppID             string `json:"app_id"`
	LastEmitTimestamp int64  `json:"last_emit_timestamp"`
	RouteEntity
}
