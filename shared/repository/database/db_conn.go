package database

import (
	"context"
	"fmt"
	"strings"
	"time"

	"xorm.io/xorm"
)

var connectionErrors = []string{
	"connection refused",
	"connection reset",
	"broken pipe",
	"invalid connection",
	"connection lost",
	"mysql server has gone away",
	"too many connections",
	"connection timeout",
}

func (db *Database) checkConnection() error {
	if db.driver == "sqlite" || db.driver == "sqlite3" {
		return nil
	}
	if db.engine == nil {
		return fmt.Errorf("database engine not initialized yet")
	}
	db.mutex.Lock()
	defer db.mutex.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	return db.engine.PingContext(ctx)
}

func (db *Database) Reconnect() error {
	if db.driver == "sqlite" || db.driver == "sqlite3" {
		return nil
	}
	if db.engine != nil {
		db.engine.Close()
	}
	var err error
	db.engine, err = xorm.NewEngine(db.driver, db.connstr, db.options...)
	return err
}

func (db *Database) EnableAutoReconnect() {
	if db.driver == "sqlite" || db.driver == "sqlite3" {
		return
	}
	go func() {
		ticker := time.NewTicker(time.Second * 60)
		defer ticker.Stop()
		for {
			select {
			case <-db.stop:
				return
			case <-ticker.C:
				if err := db.checkConnection(); err != nil {
					_ = db.Reconnect()
				}
			}
		}
	}()
}

func IsConnectionError(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	for _, connErr := range connectionErrors {
		if strings.Contains(errStr, connErr) {
			return true
		}
	}
	return false
}
