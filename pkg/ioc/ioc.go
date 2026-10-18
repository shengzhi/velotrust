package ioc

import "reflect"

type ObjectLifetime uint8

const (
	Transient ObjectLifetime = iota
	Singleton
)

func GetObject[T any]() (T, error) {
	isptr := true
	typ := reflect.TypeOf((*T)(nil)).Elem()
	if typ.Kind() == reflect.Struct {
		isptr = false
	}
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}
	v, err := defaultContainer.GetObject(typ)
	if err != nil {
		var t T
		return t, err
	}
	if !isptr {
		return reflect.ValueOf(v).Elem().Interface().(T), nil
	}
	return v.(T), nil
}

func MustGetObject[T any]() T {
	obj, err := GetObject[T]()
	if err != nil {
		panic(err)
	}
	return obj
}

func RegistObject[T any](lt ObjectLifetime) {
	typ := reflect.TypeOf((*T)(nil)).Elem()
	var creator ICreator[any]
	switch lt {
	case Singleton:
		creator = &singletonObjectCreator[T]{
			name: typ.Name(),
		}
	default:
		creator = &objectCreator[T, T]{
			name: typ.Name(),
		}
	}
	defaultContainer.AddObject(typ, creator)
}

func RegistInterface[S, T any](lt ObjectLifetime) {
	typ := reflect.TypeOf((*S)(nil)).Elem()
	var creator ICreator[any]
	switch lt {
	case Singleton:
		creator = &singletonObjectCreator[T]{
			name: typ.Name(),
		}
	default:
		creator = &objectCreator[S, T]{
			name: typ.Name(),
		}
	}
	defaultContainer.AddObject(typ, creator)
}

func RegistObjectFunc[T any](createFunc func() (*T, error), lt ObjectLifetime) {
	typ := reflect.TypeOf((*T)(nil)).Elem()
	var creator ICreator[any]
	switch lt {
	case Singleton:
		creator = &singletonObjectCreator[T]{
			name:     typ.Name(),
			createFn: createFunc,
		}
	default:
		creator = &objectCreator[T, T]{
			name:     typ.Name(),
			createFn: createFunc,
		}
	}
	defaultContainer.AddObject(typ, creator)
}

func RegistInterfaceFunc[S, T any](createFunc func() (*T, error), lt ObjectLifetime) {
	typ := reflect.TypeOf((*S)(nil)).Elem()
	var creator ICreator[any]
	switch lt {
	case Singleton:
		creator = &singletonObjectCreator[T]{
			name:     typ.Name(),
			createFn: createFunc,
		}
	default:
		creator = &objectCreator[S, T]{
			name:     typ.Name(),
			createFn: createFunc,
		}
	}
	defaultContainer.AddObject(typ, creator)
}
