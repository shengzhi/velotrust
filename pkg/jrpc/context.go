package jrpc

import (
	"errors"
	"math"
	"net"
	"net/http"
	"net/netip"
	"reflect"
	"strings"
	"sync"

	appErrors "velotrust/pkg/errors"
	"velotrust/pkg/jrpc/codec"
	jsonrpc "velotrust/pkg/jrpc/error"
)

const abortIndex int8 = math.MaxInt8 >> 1

type TransportProtocol string

const (
	TCP  TransportProtocol = "TCP"
	UDP  TransportProtocol = "UDP"
	HTTP TransportProtocol = "HTTP"
)

type RpcMethod struct {
	methodName string
	service    *serviceMap
}

func (rm RpcMethod) RequestType() reflect.Type {
	_, method, _ := rm.service.get(rm.methodName)
	return method.argsType
}

func (rm RpcMethod) ReplyType() reflect.Type {
	_, method, _ := rm.service.get(rm.methodName)
	return method.replyType
}

type Context struct {
	Protocol       TransportProtocol
	HttpRequest    *http.Request
	Writer         ResponseWriter
	Conn           net.Conn
	Identity       Identity
	AccessMode     AccessMode
	chain          handlerChain
	codecReq       codec.CodecRequest
	serviceMap     *serviceMap
	trustedProxies []netip.Prefix
	index          int8
	// This mutex protects Keys map.
	mu sync.RWMutex
	// Keys is a key/value pair exclusively for the context of each request.
	Keys     map[string]any
	response struct {
		reply any
		err   error
	}
	requestObj  any
	errorMapper ErrorMapper
	ilog        ILog
}

type HandlerFunc func(*Context)
type handlerChain []HandlerFunc

func (ctx *Context) Next() {
	ctx.index++
	for ctx.index < int8(len(ctx.chain)) {
		ctx.chain[ctx.index](ctx)
		ctx.index++
	}
}

func (ctx *Context) AbortWithError(code int, err error) {
	ctx.Abort()
	if err == nil {
		err = jsonrpc.ErrInternal()
	}
	var classified appErrors.ClassifiedError
	if errors.As(err, &classified) {
		ctx.response.err = err
		ctx.encodeError(err)
		return
	}
	ctx.encodeError(rpcErrorFromStatus(code, err))
}

func (ctx *Context) Abort() { ctx.index = abortIndex }

func (ctx *Context) RequestType() reflect.Type {
	rm := RpcMethod{methodName: ctx.ServiceName(), service: ctx.serviceMap}
	return rm.RequestType()
}

func (ctx *Context) ReplyType() reflect.Type {
	rm := RpcMethod{methodName: ctx.ServiceName(), service: ctx.serviceMap}
	return rm.ReplyType()
}

func (ctx *Context) ServiceName() string {
	if ctx.codecReq == nil {
		return ""
	}
	m, err := ctx.codecReq.Method()
	if err != nil {
		return err.Error()
	}
	return m
}

func (ctx *Context) ServiceModule() string {
	method := ctx.ServiceName()
	slice := strings.SplitN(method, ctx.serviceMap.hyphen(), 2)
	if len(slice) > 0 {
		return slice[0]
	}
	return ""
}

// ServiceMethod returns the registered method portion of the RPC name.
func (ctx *Context) ServiceMethod() string {
	if ctx == nil || ctx.serviceMap == nil {
		return ""
	}
	_, method, ok := strings.Cut(ctx.ServiceName(), ctx.serviceMap.hyphen())
	if !ok {
		return ""
	}
	return method
}

func (ctx *Context) Language() string {
	if ctx == nil || ctx.HttpRequest == nil {
		return ""
	}
	lang := ctx.HttpRequest.URL.Query().Get("lang")
	if lang == "" {
		lang = ctx.HttpRequest.Header.Get("Accept-Language")
	}
	return lang
}

func (ctx *Context) RequestID() string {
	if ctx.codecReq == nil {
		return ""
	}
	return ctx.codecReq.RequestID()
}

func (ctx *Context) RequestParams() []byte {
	if ctx.codecReq == nil {
		return nil
	}
	return ctx.codecReq.Params()
}

func (ctx *Context) RemoteIP() string {
	if ctx == nil || ctx.HttpRequest == nil {
		return ""
	}
	addr, ok := parsePeerAddress(ctx.HttpRequest.RemoteAddr)
	if !ok {
		return ""
	}
	return addr.String()
}

// ClientIP 尽最大努力实现获取客户端 IP 的算法。
// 解析 X-Real-IP 和 X-Forwarded-For 以便于反向代理（nginx 或 haproxy）可以正常工作。
func (ctx *Context) ClientIP() string {
	if ctx == nil || ctx.HttpRequest == nil {
		return ""
	}
	remote, ok := parsePeerAddress(ctx.HttpRequest.RemoteAddr)
	if !ok {
		return ""
	}
	if !ctx.isTrustedProxy(remote) {
		return remote.String()
	}
	if forwarded := strings.TrimSpace(ctx.HttpRequest.Header.Get("X-Forwarded-For")); forwarded != "" {
		current := remote
		hops := strings.Split(forwarded, ",")
		for i := len(hops) - 1; i >= 0; i-- {
			if !ctx.isTrustedProxy(current) {
				return current.String()
			}
			hop, valid := parsePeerAddress(hops[i])
			if !valid {
				return remote.String()
			}
			current = hop
		}
		return current.String()
	}
	if realIP, valid := parsePeerAddress(ctx.HttpRequest.Header.Get("X-Real-Ip")); valid {
		return realIP.String()
	}
	return remote.String()
}

func (ctx *Context) isTrustedProxy(addr netip.Addr) bool {
	addr = addr.Unmap()
	for _, prefix := range ctx.trustedProxies {
		if prefix.IsValid() && prefix.Contains(addr) {
			return true
		}
	}
	return false
}

func parsePeerAddress(value string) (netip.Addr, bool) {
	value = strings.TrimSpace(value)
	if addr, err := netip.ParseAddr(value); err == nil {
		return addr.Unmap(), true
	}
	if addrPort, err := netip.ParseAddrPort(value); err == nil {
		return addrPort.Addr().Unmap(), true
	}
	if host, _, err := net.SplitHostPort(value); err == nil {
		if addr, parseErr := netip.ParseAddr(host); parseErr == nil {
			return addr.Unmap(), true
		}
	}
	return netip.Addr{}, false
}

func (ctx *Context) RequestObj() any { return ctx.requestObj }
func (ctx *Context) Result() any     { return ctx.response.reply }
func (ctx *Context) Error() error    { return ctx.response.err }

// Set is used to store a new key/value pair exclusively for this context.
// It also lazy initializes  c.Keys if it was not used previously.
func (ctx *Context) Set(key string, value any) {
	ctx.mu.Lock()
	defer ctx.mu.Unlock()
	if ctx.Keys == nil {
		ctx.Keys = make(map[string]any)
	}

	ctx.Keys[key] = value
}

// Get returns the value for the given key, ie: (value, true).
// If the value does not exist it returns (nil, false)
func (ctx *Context) Get(key string) (value any, exists bool) {
	ctx.mu.RLock()
	defer ctx.mu.RUnlock()
	value, exists = ctx.Keys[key]
	return
}

type IdentityResolver func(*Context) (Identity, error)
type AuthHandler = IdentityResolver

var _ Identity = (*DefaultIdentity)(nil)

// Identity
type Identity interface {
	// ID 身份标识.
	ID() string
	OrgID() string
	EntityID() string
	Type() string
	// Name 账号名称.
	Name() string
	UserInfo() any
	// IsRole 是否属于指定角色.
	IsRole(...string) bool
	Roles() []string
}

type DefaultIdentity struct {
	_id       string
	_orgId    string
	_entityId string
	_type     string
	_name     string
	_roles    []string
	_userInfo any
}

func NewDefaultIdentity(id, orgId, name, entityId, types string, roles ...string) DefaultIdentity {
	return DefaultIdentity{_id: id, _orgId: orgId, _entityId: entityId, _roles: roles, _type: types, _name: name}
}
func (di DefaultIdentity) UserInfo() any             { return di._userInfo }
func (di *DefaultIdentity) SetUserInfo(userInfo any) { di._userInfo = userInfo }
func (di *DefaultIdentity) SetName(name string)      { di._name = name }
func (di DefaultIdentity) ID() string                { return di._id }
func (di DefaultIdentity) OrgID() string             { return di._orgId }
func (di DefaultIdentity) EntityID() string          { return di._entityId }
func (di DefaultIdentity) Type() string              { return di._type }
func (di DefaultIdentity) Name() string              { return di._name }
func (di DefaultIdentity) Roles() []string           { return append([]string(nil), di._roles...) }
func (di DefaultIdentity) IsRole(roles ...string) bool {
	for _, src := range di._roles {
		for _, target := range roles {
			if src == target {
				return true
			}
		}
	}
	return false
}

func rpcErrorFromStatus(code int, err error) error {
	if err == nil {
		return jsonrpc.ErrInternal()
	}
	if _, ok := err.(jsonrpc.RpcError); ok {
		return err
	}
	switch code {
	case http.StatusForbidden:
		return jsonrpc.ErrForbidden()
	case http.StatusNotFound:
		return jsonrpc.ErrMethodNotFound()
	case http.StatusBadRequest, http.StatusUnsupportedMediaType, http.StatusMethodNotAllowed:
		return jsonrpc.ErrInvalidRequest()
	default:
		return jsonrpc.ErrInternal()
	}
}
