package ioc

import (
	"fmt"
	"reflect"
	"sync"
)

type ICreator[T any] interface {
	create(IContainer, *Node) (T, error)
}

type singletonObjectCreator[T any] struct {
	name     string
	instance *T
	once     sync.Once
	createFn func() (*T, error)
	err      error
}

func (oc *singletonObjectCreator[T]) create(c IContainer, dag *Node) (any, error) {
	oc.once.Do(func() {
		if oc.createFn != nil {
			oc.instance, oc.err = oc.createFn()
		} else {
			typ := reflect.TypeOf((*T)(nil)).Elem()
			var v any
			if v, oc.err = createObject(c, typ, dag); oc.err == nil {
				oc.instance = v.(*T)
			}
		}
		if oc.err == nil && oc.instance == nil {
			oc.err = fmt.Errorf("singleton creator %q returned a nil object", oc.name)
		}
	})

	return oc.instance, oc.err
}

type objectCreator[S, T any] struct {
	name     string
	inter    S
	impl     T
	createFn func() (*T, error)
}

func (oc *objectCreator[S, T]) create(c IContainer, dag *Node) (any, error) {
	if oc.createFn != nil {
		return oc.createFn()
	}
	typ := reflect.TypeOf((*T)(nil)).Elem()
	v, err := createObject(c, typ, dag)
	if err != nil {
		var s S
		return s, err
	}
	return v, nil
}

func createObject(c IContainer, typ reflect.Type, dag *Node) (any, error) {
	if typ.Kind() != reflect.Struct {
		return nil, fmt.Errorf("[%s] is not interface or struct ", typ)
	}
	var curNode *Node
	if dag == nil {
		dag = NewNode(typ)
		curNode = dag
	} else {
		curNode = NewNode(typ)
		if err := dag.Add(curNode); err != nil {
			return nil, err
		}
	}
	value := reflect.New(typ)

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if field.PkgPath != "" {
			continue
		}
		tag := iocTag(field.Tag.Get("ioc"))
		if tag.IsEmpty() {
			continue
		}

		fieldType := field.Type
		isPtr := false
		if fieldType.Kind() == reflect.Ptr {
			fieldType = fieldType.Elem()
			isPtr = true
		}
		if fieldType.Kind() == reflect.Interface {
			isPtr = true
		}
		fieldObj, err := c.doCreateObject(curNode, fieldType)
		if err != nil {
			if tag.IsRequired() {
				return nil, fmt.Errorf("occurs error for creating field object [%s.%s]: %v", typ.Name(), fieldType.Name(), err)
			} else {
				continue
			}
		}

		fieldVal := reflect.ValueOf(fieldObj)
		if !isPtr {
			if fieldVal.Kind() != reflect.Ptr || fieldVal.IsNil() {
				if tag.IsRequired() {
					return nil, fmt.Errorf("required dependency for %s.%s is nil", typ.Name(), field.Name)
				}
				continue
			}
			fieldVal = fieldVal.Elem()
		}
		if !value.Elem().Field(i).CanSet() || !fieldVal.Type().AssignableTo(value.Elem().Field(i).Type()) {
			if tag.IsRequired() {
				return nil, fmt.Errorf("dependency for %s.%s has incompatible type %s", typ.Name(), field.Name, fieldVal.Type())
			}
			continue
		}
		value.Elem().Field(i).Set(fieldVal)
	}
	return value.Interface(), nil
}
