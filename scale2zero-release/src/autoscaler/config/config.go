package config

import (
	"strings"
	"time"

	"autoscaler/cf"
	"autoscaler/db"
	"autoscaler/helpers"
	"autoscaler/models"

	yaml "gopkg.in/yaml.v2"
)

const (
	DefaultShardID = "CF_AUTOSCALER_TO_ZERO"
)

type ServerConfig struct {
	Port int `yaml:"port"`
	// TLS       models.TLSCerts `yaml:"tls"`
	// NodeAddrs []string        `yaml:"node_addrs"`
	// NodeIndex int             `yaml:"node_index"`
}
type DBConfig struct {
	ApplicationDB   db.DatabaseConfig `yaml:"application_db"`
	ScalingEngineDB db.DatabaseConfig `yaml:"scalingengine_db"`
}
type ApiServerConfig struct {
	Server ServerConfig `yaml:"server"`
}
type NozzleConfig struct {
	RLPClientTLS *models.TLSCerts `yaml:"rlp_client_tls"`
	RLPAddr      string           `yaml:"rlp_addr"`
	ShardID      string           `yaml:"shard_id"`
}
type AppManagerConfig struct {
	AppRefreshInterval time.Duration `yaml:"app_refresh_interval"`
}
type RouteEmitterConfig struct {
	FlushInterval time.Duration `yaml:"flush_interval"`
}
type RouteManagerConfig struct {
	SendRouteInterval       time.Duration `yaml:"send_route_interval"`
	RefreshAppRouteInterval time.Duration `yaml:"refresh_app_route_interval"`
}
type RouteListenerConfig struct {
	IPAddress string `yaml:"ipaddress"`
	Port      int    `yaml:"port"`
}
type AggregatorConfig struct {
	EvaluationInterval time.Duration `yaml:"evaluation_interval"`
	AppRefreshInterval time.Duration `yaml:"app_refresh_interval"`
}
type ScalingEngineConfig struct {
	LockSize int `yaml:"lock_size"`
}
type Config struct {
	Logging            helpers.LoggingConfig `yaml:"logging"`
	DB                 DBConfig              `yaml:"db"`
	ApiServer          ApiServerConfig       `yaml:"apiserver"`
	Nozzle             NozzleConfig          `yaml:"nozzle"`
	AppManager         AppManagerConfig      `yaml:"app_manager"`
	RouteEmitter       RouteEmitterConfig    `yaml:"route_emitter"`
	RouteManager       RouteManagerConfig    `yaml:"route_manager"`
	RouteListener      RouteListenerConfig   `yaml:"route_listener"`
	Aggregator         AggregatorConfig      `yaml:"aggregator"`
	ScalingEngine      ScalingEngineConfig   `yaml:"scaling_engine"`
	CF                 cf.Config             `yaml:"cf"`
	RouteChanSize      int                   `yaml:"route_chan_size"`
	EnvelopeChanSize   int                   `yaml:"envelope_chan_size"`
	TriggerChanSize    int                   `yaml:"trigger_chan_size"`
	NozzleCount        int                   `yaml:"nozzle_count"`
	TriggerWorkerCount int                   `yaml:"trigger_worker_count"`
	CoolDownDuration   time.Duration         `yaml:"cool_down_duration"`
}

func LoadConfig(bytes []byte) (*Config, error) {
	conf := &Config{}

	err := yaml.Unmarshal(bytes, conf)
	if err != nil {
		return nil, err
	}

	conf.Logging.Level = strings.ToLower(conf.Logging.Level)
	return conf, nil
}

func (c *Config) Validate() error {

	return nil
}
