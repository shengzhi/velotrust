package database

import (
	"context"
	"database/sql"
	"fmt"
	"sync"

	"xorm.io/xorm"
	"xorm.io/xorm/schemas"
)

type Database struct {
	engine   *xorm.Engine
	driver   string
	connstr  string
	mutex    sync.Mutex
	options  []func(db *sql.DB) error
	stop     chan struct{}
	stopOnce sync.Once
}

func NewDatabase(driver string, connstr string, options ...func(db *sql.DB) error) (
	*Database, error) {
	db, err := xorm.NewEngine(driver, connstr, options...)
	if err != nil {
		return nil, err
	}
	return &Database{engine: db, driver: driver, connstr: connstr, options: options, stop: make(chan struct{})}, nil
}

func (db *Database) Engine() *xorm.Engine {
	return db.engine
}

func (db *Database) Driver() string { return db.driver }

// Ping verifies the database connection using the caller's deadline.
func (db *Database) Ping(ctx context.Context) error {
	if db == nil || db.engine == nil {
		return fmt.Errorf("database engine is not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return db.engine.PingContext(ctx)
}

// WithTransaction commits only after fn succeeds and rolls back on every
// error or panic. Callers must not retain the session after fn returns.
func (db *Database) WithTransaction(ctx context.Context, fn func(*xorm.Session) error) (err error) {
	if db == nil || db.engine == nil {
		return fmt.Errorf("database engine is not initialized")
	}
	if fn == nil {
		return fmt.Errorf("database transaction callback is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	session := db.engine.NewSession().Context(ctx)
	defer session.Close()
	if err = session.Begin(); err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = session.Rollback()
		}
	}()
	if err = fn(session); err != nil {
		return err
	}
	if err = session.Commit(); err != nil {
		return err
	}
	committed = true
	return nil
}

func (db *Database) Close() error {
	if db == nil || db.engine == nil {
		return nil
	}
	db.stopOnce.Do(func() { close(db.stop) })
	return db.engine.Close()
}

func (db *Database) NewDBTransaction() *DBTransaction {
	return NewDBTransaction(db.Engine().NewSession())
}

func (db *Database) PK(v any, pks ...any) (bool, error) {
	has, err := db.Engine().ID(schemas.NewPK(pks...)).Get(v)
	return has, err
}

func (db *Database) Get(v any, where string, args ...any) (bool, error) {
	has, err := db.Engine().Where(where, args...).Get(v)
	return has, err
}

func (db *Database) Find(v any, page Paging, orderBy []string, where string, args ...any) error {
	err := db.Engine().Where(where, args...).Asc(orderBy...).Limit(page.Limit(), page.Offset()).Find(v)
	return err
}

func (db *Database) Save(objects ...IModel) (int64, error) {
	return db.SaveContext(context.Background(), objects...)
}

// SaveContext persists models through the Save lifecycle with the supplied
// request cancellation and deadline context.
func (db *Database) SaveContext(ctx context.Context, objects ...IModel) (int64, error) {
	var count int64
	err := db.WithSaveTransaction(ctx, func(tx *DBTransaction) error {
		var err error
		count, err = tx.Save(objects...)
		return err
	})
	return count, err
}

// WithSaveTransaction runs repository reads/locks and model Save writes in a
// single transaction. Mutations inside the callback must use tx.Save.
func (db *Database) WithSaveTransaction(ctx context.Context, fn func(*DBTransaction) error) error {
	if db == nil || db.engine == nil {
		return fmt.Errorf("database engine is not initialized")
	}
	if fn == nil {
		return fmt.Errorf("save transaction callback is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	tx := NewDBTransaction(db.engine.NewSession().Context(ctx))
	defer tx.Close()
	if err := tx.OpenTxn(); err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (db *Database) SaveOne(v IModel) (int64, error) {
	return db.Save(v)
}
