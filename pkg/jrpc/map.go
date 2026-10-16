package jrpc

import (
	"fmt"
	"reflect"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"
)

var (
	// Precompute the reflect.Type of error and Context
	typeOfError   = reflect.TypeOf((*error)(nil)).Elem()
	typeOfContext = reflect.TypeOf((*Context)(nil)).Elem()
)

// ----------------------------------------------------------------------------
// service
// ----------------------------------------------------------------------------

type service struct {
	name     string                    // name of service
	rcvr     reflect.Value             // receiver of methods for the service
	rcvrType reflect.Type              // type of the receiver
	methods  map[string]*serviceMethod // registered methods
	passReq  bool
}

type serviceMethod struct {
	method    reflect.Method // receiver method
	argsType  reflect.Type   // type of the request argument
	replyType reflect.Type   // type of the response argument
}

// ----------------------------------------------------------------------------
// serviceMap
// ----------------------------------------------------------------------------

// serviceMap is a registry for services.
type serviceMap struct {
	mutex    sync.RWMutex
	services map[string]*service
	hyphen_  string
}

// register adds a new service using reflection to extract its methods.
func (m *serviceMap) register(rcvr interface{}, name string) error {
	// Setup service.
	s := &service{
		name:     name,
		rcvr:     reflect.ValueOf(rcvr),
		rcvrType: reflect.TypeOf(rcvr),
		methods:  make(map[string]*serviceMethod),
	}
	if name == "" {
		s.name = reflect.Indirect(s.rcvr).Type().Name()
		if !isExported(s.name) {
			return fmt.Errorf("rpc: type %q is not exported", s.name)
		}
	}
	if s.name == "" {
		return fmt.Errorf("rpc: no service name for type %q",
			s.rcvrType.String())
	}
	// Setup methods.
	for i := 0; i < s.rcvrType.NumMethod(); i++ {
		method := s.rcvrType.Method(i)
		mtype := method.Type

		// Method must be exported.
		if method.PkgPath != "" {
			continue
		}
		// Method needs three ins: receiver, *context, *args and tow outs: *reply, error.
		// Method sinature: func(s *receiver)(*Context, *args)(*reply,error)
		if mtype.NumIn() != 3 || mtype.NumOut() != 2 {
			continue
		}

		// First argument must be a pointer and must be http.Request.
		ctxType := mtype.In(1)
		if ctxType.Kind() != reflect.Ptr || ctxType.Elem() != typeOfContext {
			continue
		}

		// Next argument must be a pointer and must be exported.
		args := mtype.In(2)
		if args.Kind() != reflect.Ptr || !isExportedOrBuiltin(args) {
			continue
		}
		// Next argument must be a pointer and must be exported.
		reply := mtype.Out(0)
		if !isPtrOrByteSlice(reply) || !isExportedOrBuiltin(reply) {
			continue
		}

		if errType := mtype.Out(1); errType != typeOfError {
			continue
		}
		s.methods[strings.ToLower(method.Name)] = &serviceMethod{
			method:    method,
			argsType:  args.Elem(),
			replyType: reply.Elem(),
		}
	}
	if len(s.methods) == 0 {
		return fmt.Errorf("rpc: %q has no exported methods of suitable type",
			s.name)
	}
	// Add to the map.
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if m.services == nil {
		m.services = make(map[string]*service)
	} else if _, ok := m.services[s.name]; ok {
		return fmt.Errorf("rpc: service already defined: %q", s.name)
	}
	m.services[s.name] = s
	return nil
}

// get returns a registered service given a method name.
//
// The method name uses a dotted notation as in "Service.Method".
func (m *serviceMap) get(method string) (*service, *serviceMethod, error) {
	parts := strings.Split(method, m.hyphen())
	if len(parts) != 2 {
		err := fmt.Errorf("rpc: service/method request ill-formed: %q", method)
		return nil, nil, err
	}
	m.mutex.RLock()
	service := m.services[parts[0]]
	m.mutex.RUnlock()
	if service == nil {
		err := fmt.Errorf("rpc: can't find service %q", method)
		return nil, nil, err
	}
	serviceMethod := service.methods[strings.ToLower(parts[1])]
	if serviceMethod == nil {
		err := fmt.Errorf("rpc: can't find method %q", method)
		return nil, nil, err
	}
	return service, serviceMethod, nil
}

func (m *serviceMap) hyphen() string {
	if m.hyphen_ == "" {
		return "_"
	}
	return m.hyphen_
}

func (m *serviceMap) setHyphen(s string) { m.hyphen_ = s }

// isExported returns true of a string is an exported (upper case) name.
func isExported(name string) bool {
	rune, _ := utf8.DecodeRuneInString(name)
	return unicode.IsUpper(rune)
}

// isExportedOrBuiltin returns true if a type is exported or a builtin.
func isExportedOrBuiltin(t reflect.Type) bool {
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	// PkgPath will be non-empty even for an exported type,
	// so we need to check the type name as well.
	return isExported(t.Name()) || t.PkgPath() == ""
}

func isPtrOrByteSlice(t reflect.Type) bool {
	if t.Kind() == reflect.Ptr {
		return true
	}
	return t.Kind() == reflect.Slice && t.Elem().Kind() == reflect.Uint8
}
