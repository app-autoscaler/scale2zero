package sqldb

import (
	"autoscaler/db"
	"autoscaler/models"

	"code.cloudfoundry.org/lager"
	_ "github.com/lib/pq"

	"database/sql"
	"time"
)

type ScalingEngineSQLDB struct {
	dbConfig db.DatabaseConfig
	logger   lager.Logger
	sqldb    *sql.DB
}

func NewScalingEngineSQLDB(dbConfig db.DatabaseConfig, logger lager.Logger) (*ScalingEngineSQLDB, error) {
	sqldb, err := sql.Open(db.PostgresDriverName, dbConfig.URL)
	if err != nil {
		logger.Error("open-scaling-engine-db", err, lager.Data{"dbConfig": dbConfig})
		return nil, err
	}

	err = sqldb.Ping()
	if err != nil {
		sqldb.Close()
		logger.Error("ping-scaling-engine-db", err, lager.Data{"dbConfig": dbConfig})
		return nil, err
	}

	sqldb.SetConnMaxLifetime(dbConfig.ConnectionMaxLifetime)
	sqldb.SetMaxIdleConns(dbConfig.MaxIdleConnections)
	sqldb.SetMaxOpenConns(dbConfig.MaxOpenConnections)
	return &ScalingEngineSQLDB{
		dbConfig: dbConfig,
		logger:   logger,
		sqldb:    sqldb,
	}, nil
}

func (sdb *ScalingEngineSQLDB) Close() error {
	err := sdb.sqldb.Close()
	if err != nil {
		sdb.logger.Error("close-scaling-engine-db", err, lager.Data{"dbConfig": sdb.dbConfig})
		return err
	}
	return nil
}

func (sdb *ScalingEngineSQLDB) SaveScalingHistory(history *models.AppScalingHistory) error {
	query := "INSERT INTO scalinghistory" +
		"(appid, timestamp, scalingtype, status, oldinstances, newinstances, reason, message, error) " +
		" VALUES($1, $2, $3, $4, $5, $6, $7, $8, $9)"
	_, err := sdb.sqldb.Exec(query, history.AppId, history.Timestamp, history.ScalingType, history.Status,
		history.OldInstances, history.NewInstances, history.Reason, history.Message, history.Error)

	if err != nil {
		sdb.logger.Error("save-scaling-history", err, lager.Data{"query": query, "history": history})
	}
	return err
}

func (sdb *ScalingEngineSQLDB) RetrieveScalingHistories(appId string, start int64, end int64, orderType db.OrderType, includeAll bool) ([]*models.AppScalingHistory, error) {
	var orderStr string
	if orderType == db.DESC {
		orderStr = db.DESCSTR
	} else {
		orderStr = db.ASCSTR
	}

	query := "SELECT timestamp, scalingtype, status, oldinstances, newinstances, reason, message, error FROM scalinghistory WHERE" +
		" appid = $1 " +
		" AND timestamp >= $2" +
		" AND timestamp <= $3" +
		" ORDER BY timestamp " + orderStr

	if end < 0 {
		end = time.Now().UnixNano()
	}

	histories := []*models.AppScalingHistory{}
	rows, err := sdb.sqldb.Query(query, appId, start, end)
	if err != nil {
		sdb.logger.Error("retrieve-scaling-histories", err,
			lager.Data{"query": query, "appid": appId, "start": start, "end": end, "orderType": orderType})
		return nil, err
	}

	defer rows.Close()

	var timestamp int64
	var scalingType, status, oldInstances, newInstances int
	var reason, message, errorMsg string

	for rows.Next() {
		if err = rows.Scan(&timestamp, &scalingType, &status, &oldInstances, &newInstances, &reason, &message, &errorMsg); err != nil {
			sdb.logger.Error("retrieve-scaling-history-scan", err)
			return nil, err
		}

		history := models.AppScalingHistory{
			AppId:        appId,
			Timestamp:    timestamp,
			ScalingType:  models.ScalingType(scalingType),
			Status:       models.ScalingStatus(status),
			OldInstances: oldInstances,
			NewInstances: newInstances,
			Reason:       reason,
			Message:      message,
			Error:        errorMsg,
		}

		if includeAll || history.Status != models.ScalingStatusIgnored {
			histories = append(histories, &history)
		}
	}
	return histories, nil
}

func (sdb *ScalingEngineSQLDB) GetDBStatus() sql.DBStats {
	return sdb.sqldb.Stats()
}
