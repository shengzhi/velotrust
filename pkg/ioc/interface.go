package ioc

import (
	"reflect"
	"strings"
)

type IContainer interface {
	AddObject(t reflect.Type, c ICreator[any])

	GetObject(reflect.Type) (any, error)

	doCreateObject(*Node, reflect.Type) (any, error)
}

type iocTag string

func (tag iocTag) IsEmpty() bool { return tag == "" }

func (tag iocTag) IsRequired() bool {
	return strings.EqualFold("required", string(tag))
}
