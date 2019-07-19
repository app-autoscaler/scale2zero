package db

import (
	"autoscaler/models"
	"fmt"
	"time"

	"database/sql"
)

const PostgresDriverName = "postgres"

type OrderType uint8

const (
	DESC OrderType = iota
	ASC
)
const (
	DESCSTR string = "DESC"
	ASCSTR  string = "ASC"
)

var ErrAlreadyExists = fmt.Errorf("already exists")
var ErrDoesNotExist = fmt.Errorf("doesn't exist")

type DatabaseConfig struct {
	URL                   string        `yaml:"url"`
	MaxOpenConnections    int           `yaml:"max_open_connections"`
	MaxIdleConnections    int           `yaml:"max_idle_connections"`
	ConnectionMaxLifetime time.Duration `yaml:"connection_max_lifetime"`
}
type DatabaseStatus interface {
	GetDBStatus() sql.DBStats
}

type ApplicationDB interface {
	DatabaseStatus
	GetApplications() (map[string]int, error)
	SaveApplication(appId string, breachDuration int) error
	Close() error
	DeleteApplication(appId string) error
}

type ScalingEngineDB interface {
	DatabaseStatus
	SaveScalingHistory(history *models.AppScalingHistory) error
	RetrieveScalingHistories(appId string, start int64, end int64, orderType OrderType, includeAll bool) ([]*models.AppScalingHistory, error)
	Close() error
}
