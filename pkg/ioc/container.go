package ioc

import (
	"fmt"
	"reflect"
	"sync"
)

var defaultContainer IContainer = &ObjContainer{table: make(map[reflect.Type]ICreator[any]), lock: &sync.RWMutex{}}

type ObjContainer struct {
	lock  *sync.RWMutex
	table map[reflect.Type]ICreator[any]
}

func (con *ObjContainer) AddObject(t reflect.Type, c ICreator[any]) {
	con.lock.Lock()
	defer con.lock.Unlock()
	con.table[t] = c
}

func (con *ObjContainer) GetObject(t reflect.Type) (any, error) {
	return con.doCreateObject(nil, t)
}

func (con *ObjContainer) doCreateObject(dag *Node, t reflect.Type) (any, error) {
	con.lock.RLock()
	creator, ok := con.table[t]
	con.lock.RUnlock()
	if !ok {
		return nil, fmt.Errorf("[%s] is not registered", t.String())
	}
	return creator.create(con, dag)
}
