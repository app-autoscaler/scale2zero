package sqldb

import (
	"autoscaler/db"
	"database/sql"

	"code.cloudfoundry.org/lager"
	_ "github.com/lib/pq"
)

type ApplicationDB struct {
	dbConfig db.DatabaseConfig
	logger   lager.Logger
	sqldb    *sql.DB
}

func NewApplicationSQLDB(dbConfig db.DatabaseConfig, logger lager.Logger) (*ApplicationDB, error) {
	sqldb, err := sql.Open(db.PostgresDriverName, dbConfig.URL)
	if err != nil {
		logger.Error("open-application-db", err, lager.Data{"dbConfig": dbConfig})
		return nil, err
	}

	err = sqldb.Ping()
	if err != nil {
		sqldb.Close()
		logger.Error("ping-application-db", err, lager.Data{"dbConfig": dbConfig})
		return nil, err
	}

	sqldb.SetConnMaxLifetime(dbConfig.ConnectionMaxLifetime)
	sqldb.SetMaxIdleConns(dbConfig.MaxIdleConnections)
	sqldb.SetMaxOpenConns(dbConfig.MaxOpenConnections)

	return &ApplicationDB{
		dbConfig: dbConfig,
		logger:   logger,
		sqldb:    sqldb,
	}, nil
}

func (adb *ApplicationDB) Close() error {
	err := adb.sqldb.Close()
	if err != nil {
		adb.logger.Error("Close-application-db", err, lager.Data{"dbConfig": adb.dbConfig})
		return err
	}
	return nil
}

func (adb *ApplicationDB) GetApplications() (map[string]int, error) {
	appIds := make(map[string]int)
	query := "SELECT app_id,breach_duration FROM application"

	rows, err := adb.sqldb.Query(query)
	if err != nil {
		adb.logger.Error("get-appids-from-application-table", err, lager.Data{"query": query})
		return nil, err
	}
	defer rows.Close()

	var id string
	var duration int
	for rows.Next() {
		if err = rows.Scan(&id, &duration); err != nil {
			adb.logger.Error("get-appids-scan", err)
			return nil, err
		}
		appIds[id] = duration
	}
	return appIds, nil
}

func (adb *ApplicationDB) SaveApplication(appId string, breachDuration int) error {
	query := "INSERT INTO application (app_id,breach_duration) VALUES ($1, $2) " +
		"ON CONFLICT(app_id) DO UPDATE SET breach_duration=EXCLUDED.breach_duration"

	_, err := adb.sqldb.Exec(query, appId, breachDuration)
	if err != nil {
		adb.logger.Error("save-app-application", err, lager.Data{"query": query, "app_id": appId, "breach_duration": breachDuration})
	}
	return err
}

func (adb *ApplicationDB) DeleteApplication(appId string) error {
	query := "DELETE FROM application WHERE app_id = $1"
	_, err := adb.sqldb.Exec(query, appId)
	if err != nil {
		adb.logger.Error("failed-to-delete-application", err, lager.Data{"query": query, "appId": appId})
	}
	return err
}

func (adb *ApplicationDB) GetDBStatus() sql.DBStats {
	return adb.sqldb.Stats()
}
