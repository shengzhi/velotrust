package jrpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/netip"
	"os"
	"os/signal"
	"reflect"
	"runtime/debug"
	"slices"
	"strings"
	"syscall"

	autoValidator "github.com/go-playground/validator/v10"
	"github.com/gorilla/mux"
	appErrors "velotrust/pkg/errors"
	"velotrust/pkg/jrpc/codec"
	jsonrpc "velotrust/pkg/jrpc/error"
)

type validator interface {
	Validate() error
}

var validatorType = reflect.TypeOf((*validator)(nil)).Elem()

type RecoveryHandler func(*Context, any) error

type AccessMode int

const (
	AccessPublic AccessMode = iota
	AccessAuthenticated
)

type ServiceOptions struct {
	PublicMethods []string
	MethodAccess  map[string]AccessMode
}
type MethodDescriptor struct {
	Service, Method string
	Access          AccessMode
}

// NewServer creates a new RPC server.
func NewServer(options ...Option) *Server {
	s := &Server{
		codecs:          make(map[string]codec.Codec),
		services:        new(serviceMap),
		Router:          mux.NewRouter(),
		serviceHandlers: make(map[string]handlerChain),
		methodAccess:    make(map[string]AccessMode),
		Validator:       autoValidator.New(),
		defaultAccess:   AccessAuthenticated,
	}
	s.RegisterCodec(codec.NewJSONCodec(), "application/json")
	// s.RegisterCodec(codec.NewJSONCodec(), "application/json-rpc")
	for _, fn := range options {
		fn(s)
	}
	if s.ilog == nil {
		s.ilog = slog.Default()
	}
	if s.recoverHandler == nil {
		s.recoverHandler = func(_ *Context, recovered any) error {
			return fmt.Errorf("rpc panic: %v", recovered)
		}
	}
	if s.errorMapper == nil {
		s.errorMapper = defaultErrorMapper(s.errorLocalizer)
	}
	return s
}

func DefaultServer(options ...Option) *Server {
	s := NewServer(options...)
	s.Use(Logger(false))
	return s
}

// RequestInfo contains all the information we pass to before/after functions
type RequestInfo struct {
	Method     string
	Error      error
	Request    *http.Request
	StatusCode int
}

// Server serves registered RPC services using registered codecs.
type Server struct {
	Router           *mux.Router
	codecs           map[string]codec.Codec
	services         *serviceMap
	identityResolver IdentityResolver
	handlers         handlerChain
	serviceHandlers  map[string]handlerChain
	methodAccess     map[string]AccessMode
	ilog             ILog
	Validator        *autoValidator.Validate
	closeFunc        func(context.Context) error
	errorMapper      ErrorMapper
	errorLocalizer   ErrorLocalizer
	recoverHandler   RecoveryHandler
	defaultAccess    AccessMode
	trustedProxies   []netip.Prefix
}

// RegisterCodec adds a new codec to the server.
//
// Codecs are defined to process a given serialization scheme, e.g., JSON or
// XML. A codec is chosen based on the "Content-Type" header from the request,
// excluding the charset definition.
func (s *Server) RegisterCodec(codec codec.Codec, contentType string) {
	s.codecs[strings.ToLower(contentType)] = codec
}

// RegisterService registers methods with signature:
// func(*Context, *Args) (*Reply, error)
func (s *Server) RegisterService(receiver interface{}, name string, middlewares ...HandlerFunc) error {
	return s.RegisterServiceWithOptions(receiver, name, ServiceOptions{}, middlewares...)
}

func (s *Server) RegisterServiceWithOptions(receiver interface{}, name string, opts ServiceOptions, middlewares ...HandlerFunc) error {
	serviceName, err := resolveServiceName(receiver, name)
	if err != nil {
		return err
	}
	err = s.services.register(receiver, serviceName)
	if err == nil && len(middlewares) > 0 {
		key := strings.ToLower(serviceName)
		handlers := s.serviceHandlers[key]
		s.serviceHandlers[key] = append(handlers, middlewares...)
	}
	if err == nil {
		s.applyServiceOptions(serviceName, opts)
	}
	return err
}

func (s *Server) SetMethodAccess(method string, access AccessMode) {
	s.methodAccess[s.normalizeMethodKey(method)] = access
}

func (s *Server) getServiceHandler(name string) handlerChain {
	key := strings.ToLower(name)
	return s.serviceHandlers[key]
}

func (s *Server) AllServices() map[string][]string {
	result := make(map[string][]string)
	s.services.mutex.RLock()
	defer s.services.mutex.RUnlock()
	for _, svc := range s.services.services {
		methods := make([]string, 0, len(svc.methods))
		for _, m := range svc.methods {
			methods = append(methods, m.method.Name)
		}
		result[svc.name] = methods
	}
	return result
}

func (s *Server) Methods() []MethodDescriptor {
	all := s.AllServices()
	result := make([]MethodDescriptor, 0)
	for service, methods := range all {
		for _, method := range methods {
			result = append(result, MethodDescriptor{Service: service, Method: method, Access: s.methodAccessMode(s.composeMethod(service, method))})
		}
	}
	slices.SortFunc(result, func(a, b MethodDescriptor) int {
		if a.Service != b.Service {
			return strings.Compare(a.Service, b.Service)
		}
		return strings.Compare(a.Method, b.Method)
	})
	return result
}

// HasMethod returns true if the given method is registered.
//
// The method uses the configured separator, e.g. "Service_Method".
func (s *Server) HasMethod(method string) bool {
	if _, _, err := s.services.get(method); err == nil {
		return true
	}
	return false
}

func (s *Server) Use(h ...HandlerFunc) { s.handlers = append(s.handlers, h...) }

// ServeHTTP
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := &Context{
		Protocol:       HTTP,
		HttpRequest:    r,
		Writer:         &responseWriter{ResponseWriter: w, size: noWritten},
		serviceMap:     s.services,
		trustedProxies: s.trustedProxies,
		index:          -1,
		chain:          s.handlers,
		errorMapper:    s.errorMapper,
		ilog:           s.ilog,
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			s.logError("panic in ServeHTTP", "panic", recovered, "stack", string(debug.Stack()))
			ctx.Abort()
			ctx.encodeError(s.recoverHandler(ctx, recovered))
		}
	}()

	if r.Method != "POST" {
		ctx.encodeError(jsonrpc.ErrInvalidRequest())
		return
	}
	contentType := r.Header.Get("Content-Type")
	idx := strings.Index(contentType, ";")
	if idx != -1 {
		contentType = contentType[:idx]
	}
	var codec codec.Codec
	if contentType == "" && len(s.codecs) == 1 {
		// If Content-Type is not set and only one codec has been registered,
		// then default to that codec.
		for _, c := range s.codecs {
			codec = c
		}
	} else if codec = s.codecs[strings.ToLower(contentType)]; codec == nil {
		ctx.encodeError(jsonrpc.ErrInvalidRequest())
		return
	}
	ctx.codecReq = codec.NewRequest(r)
	service, method, err := s.getService(ctx)
	if err != nil {
		ctx.encodeError(requestError(method, err))
		return
	}
	handlers := s.getServiceHandler(service.name)
	ctx.chain = append(ctx.chain, handlers...)
	ctx.chain = append(ctx.chain, s.rpcHandler)

	ctx.AccessMode = s.methodAccessMode(method)
	// Public methods must remain callable when a client sends an expired or
	// otherwise invalid bearer token. Resolve identity only for methods whose
	// access policy requires authentication; public handlers do not consume it.
	if s.identityResolver != nil && ctx.AccessMode == AccessAuthenticated {
		iden, err := s.identityResolver(ctx)
		if err != nil {
			if ctx.ilog != nil {
				ctx.ilog.Debug("rpc identity resolution failed", "request_id", ctx.RequestID(), "method", ctx.ServiceName(), "error", err)
			}
			ctx.encodeError(jsonrpc.ErrUnauthorized())
			return
		}
		ctx.Identity = iden
	}
	if ctx.AccessMode == AccessAuthenticated && ctx.Identity == nil {
		if ctx.ilog != nil {
			ctx.ilog.Debug("authenticated rpc request has no identity", "request_id", ctx.RequestID(), "method", ctx.ServiceName())
		}
		ctx.encodeError(jsonrpc.ErrUnauthorized())
		return
	}
	ctx.Next()
}

func (s *Server) getService(c *Context) (*service, string, error) {
	method, errMethod := c.codecReq.Method()
	if errMethod != nil {
		return nil, "", errMethod
	}
	serviceSpec, _, errGet := s.services.get(method)
	return serviceSpec, method, errGet
}

func (s *Server) rpcHandler(c *Context) {
	defer func() {
		if recovered := recover(); recovered != nil {
			s.logError("panic in rpcHandler", "panic", recovered, "stack", string(debug.Stack()))
			c.Abort()
			c.encodeError(s.recoverHandler(c, recovered))
		}
	}()
	// Get service method to be called.
	method, errMethod := c.codecReq.Method()
	if errMethod != nil {
		c.Abort()
		c.encodeError(requestError("", errMethod))
		return
	}
	serviceSpec, methodSpec, errGet := s.services.get(method)
	if errGet != nil {
		c.Abort()
		c.encodeError(jsonrpc.ErrMethodNotFound())
		return
	}
	// Decode the args.
	args := reflect.New(methodSpec.argsType)
	argV := args.Interface()
	if errRead := c.codecReq.ReadRequest(argV); errRead != nil {
		c.Abort()
		c.encodeError(decodeError(errRead))
		return
	}
	c.requestObj = argV
	if err := s.Validator.Struct(argV); err != nil {
		c.encodeError(appErrors.NewClientError("INVALID_PARAMS", validationFields(err)))
		c.Abort()
		return
	}
	if v, ok := argV.(validator); ok {
		if err := v.Validate(); err != nil {
			c.encodeError(appErrors.NewClientError("INVALID_PARAMS", validationFields(err)))
			c.Abort()
			return
		}
	}
	// Call the service method.
	// reply := reflect.New(methodSpec.replyType)
	// omit the HTTP request if the service method doesn't accept it
	values := methodSpec.method.Func.Call([]reflect.Value{
		serviceSpec.rcvr,
		reflect.ValueOf(c),
		args,
	})
	reply := values[0]
	// Cast the result to error if needed.
	var errResult error
	errInter := values[1].Interface()
	if errInter != nil {
		errResult = errInter.(error)
	}
	c.response.err = errResult

	c.Writer.Header().Set("x-content-type-options", "nosniff")
	if c.Writer.Written() {
		return
	}
	replyType := reply.Type()
	if replyType.Kind() == reflect.Ptr {
		replyType = replyType.Elem()
	}
	if errResult != nil {
		c.response.reply = nil
		c.encodeError(errResult)
		return
	}
	// 字节数组
	if replyType.Kind() == reflect.Slice && replyType.Elem().Kind() == reflect.Uint8 {
		bytestream := reply.Bytes()
		c.response.reply = bytestream
		fileCodec := codec.FileCodec{}
		if errWrite := fileCodec.WriteResponse(c.Writer, bytestream, errResult); errWrite != nil {
			c.AbortWithError(400, errWrite)
		}
	} else {
		c.response.reply = reply.Interface()
		// Encode the response.
		if errWrite := c.codecReq.WriteResponse(c.Writer, c.response.reply, errResult); errWrite != nil {
			c.AbortWithError(400, errWrite)
		}
	}
}

func validationFields(err error) map[string]string {
	fields := make(map[string]string)
	if errs, ok := err.(autoValidator.ValidationErrors); ok {
		for _, validationErr := range errs {
			value := validationErr.Tag()
			if validationErr.Param() != "" {
				value += ":" + validationErr.Param()
			}
			fields[validationErr.Field()] = value
		}
	}
	return fields
}

func (s *Server) ListenHTTP(port int, path string) error {
	return s.ListenHTTPWithHandler(port, path, nil)
}

// ListenHTTPWithHandler wraps the router at the HTTP boundary, before RPC
// decoding. It is intended for concerns such as CORS and proxy middleware.
func (s *Server) ListenHTTPWithHandler(port int, path string, wrap func(http.Handler) http.Handler) error {
	s.Router.Handle(path, s)
	var handler http.Handler = s.Router
	if wrap != nil {
		handler = wrap(handler)
	}
	webServer := http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: handler,
	}
	s.closeFunc = webServer.Shutdown
	return webServer.ListenAndServe()
}

func (s *Server) ListenHTTPWithGraceful(port int, path string) error {
	s.Router.Handle(path, s)
	webServer := http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: s.Router,
	}
	idleConnectionsClosed := make(chan struct{})
	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT, os.Interrupt)
		<-quit
		if err := webServer.Shutdown(context.Background()); err != nil {
			s.logError("HTTP server shutdown error", "error", err)
		}
		close(idleConnectionsClosed)
	}()
	if err := webServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	<-idleConnectionsClosed
	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	if s.closeFunc != nil {
		return s.closeFunc(ctx)
	}
	return nil
}

func (s *Server) logError(msg string, args ...interface{}) {
	if s.ilog != nil {
		s.ilog.Error(msg, args...)
	}
}

func (s *Server) methodAccessMode(method string) AccessMode {
	if access, ok := s.methodAccess[s.normalizeMethodKey(method)]; ok {
		return access
	}
	return s.defaultAccess
}

func (s *Server) applyServiceOptions(serviceName string, opts ServiceOptions) {
	for _, methodName := range opts.PublicMethods {
		s.SetMethodAccess(s.composeMethod(serviceName, methodName), AccessPublic)
	}
	for methodName, access := range opts.MethodAccess {
		s.SetMethodAccess(s.composeMethod(serviceName, methodName), access)
	}
}

func (s *Server) composeMethod(serviceName, methodName string) string {
	if strings.Contains(methodName, s.services.hyphen()) {
		return methodName
	}
	return serviceName + s.services.hyphen() + methodName
}

func (s *Server) normalizeMethodKey(method string) string {
	return strings.ToLower(strings.TrimSpace(method))
}

func resolveServiceName(receiver interface{}, name string) (string, error) {
	if name != "" {
		return name, nil
	}
	rcvr := reflect.ValueOf(receiver)
	resolved := reflect.Indirect(rcvr).Type().Name()
	if resolved == "" {
		return "", fmt.Errorf("rpc: no service name for type %q", reflect.TypeOf(receiver).String())
	}
	if !isExported(resolved) {
		return "", fmt.Errorf("rpc: type %q is not exported", resolved)
	}
	return resolved, nil
}

func requestError(method string, err error) *jsonrpc.Error {
	if err == nil {
		return jsonrpc.ErrInternal()
	}
	if rpcErr := parseError(err); rpcErr != nil {
		return rpcErr
	}
	if method == "" {
		return jsonrpc.ErrInvalidRequest()
	}
	return jsonrpc.ErrMethodNotFound()
}

func decodeError(err error) *jsonrpc.Error {
	if err == nil {
		return jsonrpc.ErrInternal()
	}
	if rpcErr := parseError(err); rpcErr != nil {
		return rpcErr
	}
	return jsonrpc.ErrInvalidRequest()
}

func parseError(err error) *jsonrpc.Error {
	var syntaxErr *json.SyntaxError
	var typeErr *json.UnmarshalTypeError
	switch {
	case errors.As(err, &syntaxErr), errors.As(err, &typeErr), errors.Is(err, io.ErrUnexpectedEOF):
		return jsonrpc.ErrParse()
	}
	return nil
}
