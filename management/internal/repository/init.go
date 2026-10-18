package db

import (
	"fmt"
	"strings"
	"sync"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	_ "modernc.org/sqlite"
	servicecontext "velotrust/management/internal/context"
	"velotrust/shared/repository/database"
	"xorm.io/xorm"
	"xorm.io/xorm/schemas"
)

var defaultDB *database.Database
var mu sync.Mutex

// Initialize configures the process-wide database connection. Supported
// drivers are mysql, postgres (pgx) and sqlite.
func Initialize(driver, dsn string, showSQL bool) error {
	driver, err := normalizeDriver(driver)
	if err != nil {
		return err
	}
	if strings.TrimSpace(dsn) == "" {
		return fmt.Errorf("database dsn is required")
	}

	mu.Lock()
	defer mu.Unlock()
	if defaultDB != nil {
		if err := defaultDB.Close(); err != nil {
			return err
		}
	}
	defaultDB, err = database.NewDatabase(driver, dsn)
	if err != nil {
		return err
	}
	if driver == "sqlite" {
		defaultDB.Engine().DB().SetMaxOpenConns(1)
		if _, err := defaultDB.Engine().Exec("PRAGMA foreign_keys = ON"); err != nil {
			return err
		}
	}
	// database/sql already replaces unhealthy pooled connections. Replacing the
	// XORM engine from a background goroutine can race with active repositories.
	defaultDB.Engine().ShowSQL(showSQL)
	return defaultDB.Engine().Ping()
}

func normalizeDriver(driver string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(driver)) {
	case "mysql":
		return "mysql", nil
	case "postgres", "postgresql", "pgx":
		return "postgres", nil
	case "sqlite", "sqlite3":
		return "sqlite", nil
	default:
		return "", fmt.Errorf("unsupported database driver %q", driver)
	}
}

func DB() *database.Database {
	return defaultDB
}

func Engine() *xorm.Engine {
	return defaultDB.Engine()
}

func Close() error {
	mu.Lock()
	defer mu.Unlock()
	if defaultDB == nil {
		return nil
	}
	err := defaultDB.Close()
	defaultDB = nil
	return err
}

func PK[T any](pks ...interface{}) (T, bool, error) {
	var v T
	has, err := DB().Engine().ID(schemas.NewPK(pks...)).Get(&v)
	return v, has, err
}

func Get[T any](where string, args ...interface{}) (T, bool, error) {
	var t T
	has, err := Engine().Where(where, args...).Get(&t)
	if err != nil {
		return t, false, err
	}
	return t, has, nil
}

func All[T any](where string, args ...interface{}) ([]T, error) {
	var t []T
	err := Engine().Where(where, args...).Find(&t)
	if err != nil {
		return nil, err
	}
	return t, nil
}

func Find[T any](page database.Page, where string, args ...interface{}) ([]T, error) {
	var t []T
	err := Engine().Where(where, args...).Limit(page.Limit(), page.Offset()).Find(&t)
	if err != nil {
		return nil, err
	}
	return t, nil
}

func FindAndSort[T any](page database.Page, sort string, where string, args ...interface{}) ([]T, error) {
	var t []T
	err := Engine().Where(where, args...).Limit(page.Limit(), page.Offset()).OrderBy(sort).Find(&t)
	if err != nil {
		return nil, err
	}
	return t, nil
}

func Save(objects ...database.IModel) (int64, error) {
	n, err := DB().Save(objects...)
	if err != nil {
		return n, err
	}
	return n, nil
}

func SaveContext(ctx *servicecontext.Context, objects ...database.IModel) (int64, error) {
	return DB().SaveContext(ctx, objects...)
}

func SaveSlice[T database.IModel](objects []T) (int64, error) {
	models := make([]database.IModel, len(objects))
	for i, v := range objects {
		models[i] = v
	}
	return Save(models...)
}

func IsDeuplicateErr(err error) bool {
	return strings.Contains(err.Error(), "Duplicate entry")
}
