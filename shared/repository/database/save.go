package database

import (
	"fmt"
	"reflect"
	"strings"

	"xorm.io/xorm/schemas"
)

func saveObject(txn *DBTransaction, obj IModel) (int64, error) {
	if obj == nil {
		return 0, fmt.Errorf("cannot save a nil model")
	}
	if table, ok := obj.(ITableName); !ok || strings.TrimSpace(table.TableName()) == "" {
		return 0, fmt.Errorf("model %T must provide a table name", obj)
	}
	applyConditions := func() {
		if conditional, ok := obj.(interface{ SaveConditions() []SaveCondition }); ok {
			for _, condition := range conditional.SaveConditions() {
				if strings.TrimSpace(condition.SQL) != "" {
					txn.And(condition.SQL, condition.Args...)
				}
			}
		}
	}
	switch obj.State() {
	case ModelState_New:
		return txn.InsertOne(obj)
	case ModelState_Modified:
		values := obj.PKValues()
		if len(values) == 0 {
			var err error
			if values, err = getPKValue(obj); err != nil {
				return 0, err
			}
		}
		txn.ID(schemas.NewPK(values...)).And("version_no=?", obj.Version())
		applyConditions()
		return txn.Cols(obj.UpdateCols()...).Update(obj)
	case ModelState_SoftDeleted:
		values := obj.PKValues()
		if len(values) == 0 {
			var err error
			if values, err = getPKValue(obj); err != nil {
				return 0, err
			}
		}
		txn.ID(schemas.NewPK(values...)).And("version_no=?", obj.Version())
		applyConditions()
		return txn.Cols(obj.UpdateCols()...).Update(obj)
	case ModelState_HardDeleted:
		values := obj.PKValues()
		if len(values) == 0 {
			var err error
			if values, err = getPKValue(obj); err != nil {
				return 0, err
			}
		}
		delObj := reflect.New(reflect.TypeOf(obj).Elem()).Interface()
		txn.ID(schemas.NewPK(values...)).And("version_no=?", obj.Version())
		applyConditions()
		return txn.Delete(delObj)
	}
	return 0, fmt.Errorf("unknown model state")
}

func getPKValue(model interface{}) ([]interface{}, error) {
	v := reflect.ValueOf(model)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	var values []interface{}
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		tag := field.Tag.Get("xorm")
		if tag != "" && strings.Contains(tag, "pk") {
			values = append(values, v.Field(i).Interface())
		}
	}
	if len(values) > 0 {
		return values, nil
	}
	return nil, fmt.Errorf("no primary key field found")
}
