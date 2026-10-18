package database

import (
	"fmt"
	"reflect"
	"sync"

	"xorm.io/xorm"
)

type DBTransaction struct {
	*xorm.Session
	once             sync.Once
	hasTxn, finished bool
	beginErr         error
	savedModels      []IModel
}

func NewDBTransaction(sess *xorm.Session) *DBTransaction {
	return &DBTransaction{
		Session: sess,
	}
}

func (tn *DBTransaction) OpenTxn() (err error) {
	tn.once.Do(func() {
		tn.beginErr = tn.Session.Begin()
		if tn.beginErr == nil {
			tn.hasTxn = true
		}
	})
	return tn.beginErr
}

func (tn *DBTransaction) Rollback() error {
	if !tn.hasTxn || tn.finished {
		return nil
	}
	tn.finished = true
	return tn.Session.Rollback()
}

func (tn *DBTransaction) Commit() error {
	if !tn.hasTxn || tn.finished {
		return nil
	}
	if err := tn.Session.Commit(); err != nil {
		_ = tn.Session.Rollback()
		tn.finished = true
		return err
	}
	tn.finished = true
	for _, model := range tn.savedModels {
		model.SetModelState(ModelState_None)
	}
	tn.savedModels = nil
	return nil
}

func (tn *DBTransaction) Close() error {
	if tn.hasTxn && !tn.finished {
		_ = tn.Rollback()
	}
	return tn.Session.Close()
}

func (tn *DBTransaction) Save(objects ...IModel) (int64, error) {
	var cnt int64
	for i := range objects {
		value := reflect.ValueOf(objects[i])
		if !value.IsValid() || (value.Kind() == reflect.Ptr && value.IsNil()) {
			return 0, fmt.Errorf("cannot save a nil model")
		}
		if n, err := saveObject(tn, objects[i]); err != nil {
			return 0, err
		} else {
			if n != 1 {
				table := "<unknown>"
				if named, ok := objects[i].(ITableName); ok {
					table = named.TableName()
				}
				if n == 0 && objects[i].State() != ModelState_New {
					return 0, fmt.Errorf("%w: save table %s affected no rows", ErrVersionConflict, table)
				}
				return 0, fmt.Errorf("save table %s affected %d rows", table, n)
			}
			cnt += n
			tn.savedModels = append(tn.savedModels, objects[i])
		}
	}
	return cnt, nil
}

type ITableName interface {
	TableName() string
}
