// SPDX-License-Identifier: BSD-3-Clause

// Package dnsproxy implements the client's local suffix-routing DNS proxy.
package dnsproxy

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net"
	"net/netip"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/net/idna"

	clientdns "velotrust/client/internal/dns"

	"github.com/miekg/dns"
)

const (
	listenPort       = 53
	queryTimeout     = 5 * time.Second
	maxCacheEntries  = 2048
	maxCacheDuration = 5 * time.Minute
	healthDomain     = "vtrust-dns-check.invalid."
)

type routeRule struct {
	suffix    string
	resolvers []netip.AddrPort
	group     int
}

type routeSnapshot struct {
	rules    []routeRule
	original []netip.AddrPort
	version  uint64
}

type cacheEntry struct {
	response *dns.Msg
	expires  time.Time
}

// Proxy accepts DNS over UDP and TCP on loopback. Internal names use only
// their matching enterprise resolver group; other names use the captured
// system DNS servers.
type Proxy struct {
	state atomic.Pointer[routeSnapshot]

	cacheMu sync.Mutex
	cache   map[string]cacheEntry

	servers      []*dns.Server
	conns        []net.Listener
	packets      []net.PacketConn
	serverErrors chan error
	done         chan struct{}
	serverWG     sync.WaitGroup
	stopOnce     sync.Once
	closing      atomic.Bool
	errMu        sync.Mutex
	terminalErr  error
}

// New validates and snapshots the enterprise suffix policy. originalDNS may
// change as the operating system moves between networks.
func New(policy clientdns.Config, originalDNS []netip.AddrPort) (*Proxy, error) {
	if !policy.Enabled || len(policy.ResolverGroups) == 0 {
		return nil, errors.New("DNS proxy requires an enabled profile with resolver groups")
	}

	rules := make([]routeRule, 0)
	for groupIndex, group := range policy.ResolverGroups {
		if len(group.Domains) == 0 || len(group.Resolvers) == 0 {
			return nil, fmt.Errorf("DNS resolver group %d must contain domains and resolvers", groupIndex)
		}
		for _, rawDomain := range group.Domains {
			domain, err := clientdns.NormalizeDomain(rawDomain)
			if err != nil {
				return nil, err
			}
			rules = append(rules, routeRule{
				suffix:    domain,
				resolvers: append([]netip.AddrPort(nil), group.Resolvers...),
				group:     groupIndex,
			})
		}
	}
	sort.Slice(rules, func(i, j int) bool {
		if len(rules[i].suffix) != len(rules[j].suffix) {
			return len(rules[i].suffix) > len(rules[j].suffix)
		}
		return rules[i].suffix < rules[j].suffix
	})
	proxy := &Proxy{
		cache:        make(map[string]cacheEntry),
		serverErrors: make(chan error, 4),
		done:         make(chan struct{}),
	}
	proxy.state.Store(&routeSnapshot{rules: rules, original: canonicalUpstreams(originalDNS), version: 1})
	return proxy, nil
}

// UpdateOriginalDNS atomically replaces the system DNS fallback set. An empty
// set is valid and causes external queries to return SERVFAIL until resolvers
// become available again.
func (p *Proxy) UpdateOriginalDNS(upstreams []netip.AddrPort) {
	upstreams = canonicalUpstreams(upstreams)
	for {
		old := p.state.Load()
		if old == nil {
			return
		}
		if equalUpstreams(old.original, upstreams) {
			return
		}
		next := &routeSnapshot{
			rules:    old.rules,
			original: upstreams,
			version:  old.version + 1,
		}
		if p.state.CompareAndSwap(old, next) {
			p.clearCache()
			slog.Info("updated original DNS upstreams", "upstream_count", len(upstreams))
			return
		}
	}
}

// Start binds both IP families and both DNS transports before returning. The
// DNS proxy is available only on loopback, never on a LAN interface.
func (p *Proxy) Start() error {
	for _, endpoint := range []struct {
		network string
		address string
	}{
		{network: "udp4", address: net.JoinHostPort("127.0.0.1", fmt.Sprint(listenPort))},
		{network: "tcp4", address: net.JoinHostPort("127.0.0.1", fmt.Sprint(listenPort))},
		{network: "udp6", address: net.JoinHostPort("::1", fmt.Sprint(listenPort))},
		{network: "tcp6", address: net.JoinHostPort("::1", fmt.Sprint(listenPort))},
	} {
		server := &dns.Server{Net: endpoint.network, Handler: p}
		if strings.HasPrefix(endpoint.network, "udp") {
			packet, err := net.ListenPacket(endpoint.network, endpoint.address)
			if err != nil {
				p.closeUnstarted()
				return fmt.Errorf("listen for local DNS on %s: %w", endpoint.address, err)
			}
			server.PacketConn = packet
			p.packets = append(p.packets, packet)
		} else {
			listener, err := net.Listen(endpoint.network, endpoint.address)
			if err != nil {
				p.closeUnstarted()
				return fmt.Errorf("listen for local DNS on %s: %w", endpoint.address, err)
			}
			server.Listener = listener
			p.conns = append(p.conns, listener)
		}
		p.servers = append(p.servers, server)
	}
	p.serverWG.Add(len(p.servers))
	for _, server := range p.servers {
		server := server
		go func() {
			defer p.serverWG.Done()
			p.serverErrors <- server.ActivateAndServe()
		}()
	}
	go p.monitorServers()
	slog.Info("local DNS proxy started", "listen_addresses", []string{"127.0.0.1:53", "[::1]:53"})
	return nil
}

// Done closes if a listener exits. A nil result means Close was requested.
func (p *Proxy) Done() <-chan struct{} { return p.done }

// Err reports an unexpected listener failure after Done has closed.
func (p *Proxy) Err() error {
	p.errMu.Lock()
	defer p.errMu.Unlock()
	return p.terminalErr
}

// Close stops every listener and waits for active DNS handlers to finish.
func (p *Proxy) Close() error {
	p.closing.Store(true)
	p.stopServers()
	<-p.done
	return p.Err()
}

func (p *Proxy) monitorServers() {
	err := <-p.serverErrors
	if !p.closing.Load() {
		if err == nil {
			err = errors.New("DNS proxy listener stopped unexpectedly")
		}
		p.errMu.Lock()
		p.terminalErr = err
		p.errMu.Unlock()
		p.closing.Store(true)
	}
	p.stopServers()
	p.serverWG.Wait()
	close(p.done)
}

func (p *Proxy) stopServers() {
	p.stopOnce.Do(p.closeServers)
}

func (p *Proxy) closeServers() {
	for _, server := range p.servers {
		if err := server.Shutdown(); err != nil {
			if server.PacketConn != nil {
				_ = server.PacketConn.Close()
			}
			if server.Listener != nil {
				_ = server.Listener.Close()
			}
		}
	}
}

func (p *Proxy) closeUnstarted() {
	for _, packet := range p.packets {
		_ = packet.Close()
	}
	for _, listener := range p.conns {
		_ = listener.Close()
	}
}

func (p *Proxy) ServeDNS(writer dns.ResponseWriter, request *dns.Msg) {
	if request == nil || len(request.Question) != 1 {
		p.writeError(writer, request, dns.RcodeFormatError)
		return
	}
	if request.Opcode != dns.OpcodeQuery {
		p.writeError(writer, request, dns.RcodeNotImplemented)
		return
	}
	question := request.Question[0]
	qname, err := normalizeQueryName(question.Name)
	if err != nil {
		p.writeError(writer, request, dns.RcodeFormatError)
		return
	}
	if strings.HasSuffix(qname, healthDomain) {
		p.writeHealthResponse(writer, request)
		return
	}
	name := "."
	if qname != "." {
		name = qname
	}
	snapshot := p.state.Load()
	if snapshot == nil {
		p.writeError(writer, request, dns.RcodeServerFailure)
		return
	}
	upstreams, routeKind, routeID := chooseUpstreams(snapshot, name)
	dnssecOK := false
	if opt := request.IsEdns0(); opt != nil {
		dnssecOK = opt.Do()
	}
	cacheKey := fmt.Sprintf("%d:%s:%d:%d:%s:rd=%t:cd=%t:do=%t", snapshot.version, name, question.Qtype, question.Qclass, routeID, request.RecursionDesired, request.CheckingDisabled, dnssecOK)
	if response := p.cacheGet(cacheKey); response != nil {
		response.Id = request.Id
		_ = writer.WriteMsg(response)
		return
	}
	started := time.Now()
	response, err := exchange(request, upstreams)
	if err != nil {
		slog.Warn("DNS upstream query failed", "route", routeKind, "name", name, "qtype", dns.TypeToString[question.Qtype], "error", err)
		p.writeError(writer, request, dns.RcodeServerFailure)
		return
	}
	response.Id = request.Id
	response.Compress = request.Compress
	if !hasOpaqueEDNSOptions(request) {
		p.cachePut(cacheKey, response)
	}
	slog.Debug("DNS query forwarded", "route", routeKind, "name", name, "qtype", dns.TypeToString[question.Qtype], "rcode", dns.RcodeToString[response.Rcode], "duration", time.Since(started))
	_ = writer.WriteMsg(response)
}

func chooseUpstreams(snapshot *routeSnapshot, name string) ([]netip.AddrPort, string, string) {
	for _, rule := range snapshot.rules {
		if dns.IsSubDomain(dns.Fqdn(rule.suffix), name) {
			return rule.resolvers, "enterprise", fmt.Sprintf("group-%d:%s", rule.group, rule.suffix)
		}
	}
	return snapshot.original, "original", "original"
}

// normalizeQueryName accepts all valid DNS label forms, including service
// labels such as _ldap._tcp, while converting international labels to IDNA.
func normalizeQueryName(name string) (string, error) {
	name = dns.Fqdn(name)
	if _, ok := dns.IsDomainName(name); !ok {
		return "", fmt.Errorf("invalid DNS query name %q", name)
	}
	labels := dns.SplitDomainName(name)
	for i, label := range labels {
		if !strings.ContainsFunc(label, func(r rune) bool { return r > 127 }) {
			labels[i] = strings.ToLower(label)
			continue
		}
		if strings.Contains(label, "\\") {
			return "", fmt.Errorf("invalid escaped international DNS label %q", label)
		}
		ascii, err := idna.Lookup.ToASCII(label)
		if err != nil {
			return "", fmt.Errorf("normalize DNS query label %q: %w", label, err)
		}
		labels[i] = strings.ToLower(ascii)
	}
	if len(labels) == 0 {
		return ".", nil
	}
	return strings.Join(labels, ".") + ".", nil
}

func exchange(request *dns.Msg, upstreams []netip.AddrPort) (*dns.Msg, error) {
	if len(upstreams) == 0 {
		return nil, errors.New("no resolver is available for the selected DNS route")
	}
	ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
	defer cancel()
	var lastErr error
	for _, upstream := range upstreams {
		udp := &dns.Client{Net: "udp", Timeout: queryTimeout}
		response, _, err := udp.ExchangeContext(ctx, request.Copy(), upstream.String())
		if err != nil {
			lastErr = err
			continue
		}
		if response.Truncated {
			tcp := &dns.Client{Net: "tcp", Timeout: queryTimeout}
			response, _, err = tcp.ExchangeContext(ctx, request.Copy(), upstream.String())
			if err != nil {
				lastErr = err
				continue
			}
		}
		if response.Rcode == dns.RcodeServerFailure {
			lastErr = fmt.Errorf("resolver %s returned SERVFAIL", upstream)
			continue
		}
		return response, nil
	}
	if lastErr == nil {
		lastErr = errors.New("all configured DNS resolvers failed")
	}
	return nil, lastErr
}

func (p *Proxy) writeError(writer dns.ResponseWriter, request *dns.Msg, rcode int) {
	if request == nil {
		return
	}
	response := new(dns.Msg)
	response.SetRcode(request, rcode)
	response.RecursionAvailable = true
	_ = writer.WriteMsg(response)
}

func (p *Proxy) writeHealthResponse(writer dns.ResponseWriter, request *dns.Msg) {
	response := new(dns.Msg)
	response.SetReply(request)
	response.Authoritative = true
	question := request.Question[0]
	switch question.Qtype {
	case dns.TypeA:
		response.Answer = []dns.RR{&dns.A{Hdr: dns.RR_Header{Name: question.Name, Rrtype: dns.TypeA, Class: question.Qclass, Ttl: 1}, A: net.IPv4(127, 0, 0, 1)}}
	case dns.TypeAAAA:
		response.Answer = []dns.RR{&dns.AAAA{Hdr: dns.RR_Header{Name: question.Name, Rrtype: dns.TypeAAAA, Class: question.Qclass, Ttl: 1}, AAAA: net.ParseIP("::1")}}
	}
	_ = writer.WriteMsg(response)
}

func canonicalUpstreams(upstreams []netip.AddrPort) []netip.AddrPort {
	seen := make(map[netip.AddrPort]struct{}, len(upstreams))
	result := make([]netip.AddrPort, 0, len(upstreams))
	for _, upstream := range upstreams {
		if !upstream.IsValid() || upstream.Port() == 0 || upstream.Addr().IsUnspecified() || upstream.Addr().IsMulticast() || upstream.Addr().IsLoopback() || upstream.Addr().Is4In6() {
			continue
		}
		upstream = netip.AddrPortFrom(upstream.Addr().Unmap(), upstream.Port())
		if _, exists := seen[upstream]; exists {
			continue
		}
		seen[upstream] = struct{}{}
		result = append(result, upstream)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].String() < result[j].String() })
	return result
}

func equalUpstreams(left, right []netip.AddrPort) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func (p *Proxy) cacheGet(key string) *dns.Msg {
	p.cacheMu.Lock()
	defer p.cacheMu.Unlock()
	entry, exists := p.cache[key]
	if !exists {
		return nil
	}
	remaining := time.Until(entry.expires)
	if remaining <= 0 {
		delete(p.cache, key)
		return nil
	}
	response := entry.response.Copy()
	seconds := uint32(math.Ceil(remaining.Seconds()))
	for _, section := range [][]dns.RR{response.Answer, response.Ns, response.Extra} {
		for _, rr := range section {
			if _, ok := rr.(*dns.OPT); ok {
				continue
			}
			rr.Header().Ttl = seconds
		}
	}
	return response
}

func hasOpaqueEDNSOptions(request *dns.Msg) bool {
	opt := request.IsEdns0()
	return opt != nil && len(opt.Option) != 0
}

func (p *Proxy) cachePut(key string, response *dns.Msg) {
	if response == nil || (response.Rcode != dns.RcodeSuccess && response.Rcode != dns.RcodeNameError) {
		return
	}
	ttl, ok := responseTTL(response)
	if !ok || ttl == 0 {
		return
	}
	if ttl > uint32(maxCacheDuration/time.Second) {
		ttl = uint32(maxCacheDuration / time.Second)
	}
	p.cacheMu.Lock()
	defer p.cacheMu.Unlock()
	if len(p.cache) >= maxCacheEntries {
		p.removeExpiredLocked()
	}
	if len(p.cache) >= maxCacheEntries {
		var oldestKey string
		var oldest time.Time
		for candidate, entry := range p.cache {
			if oldestKey == "" || entry.expires.Before(oldest) {
				oldestKey, oldest = candidate, entry.expires
			}
		}
		delete(p.cache, oldestKey)
	}
	p.cache[key] = cacheEntry{response: response.Copy(), expires: time.Now().Add(time.Duration(ttl) * time.Second)}
}

func responseTTL(response *dns.Msg) (uint32, bool) {
	var minTTL uint32
	found := false
	for _, section := range [][]dns.RR{response.Answer, response.Ns, response.Extra} {
		for _, rr := range section {
			if _, ok := rr.(*dns.OPT); ok {
				continue
			}
			ttl := rr.Header().Ttl
			if soa, ok := rr.(*dns.SOA); ok && soa.Minttl < ttl {
				ttl = soa.Minttl
			}
			if !found || ttl < minTTL {
				minTTL, found = ttl, true
			}
		}
	}
	return minTTL, found
}

func (p *Proxy) clearCache() {
	p.cacheMu.Lock()
	clear(p.cache)
	p.cacheMu.Unlock()
}

func (p *Proxy) removeExpiredLocked() {
	now := time.Now()
	for key, entry := range p.cache {
		if !now.Before(entry.expires) {
			delete(p.cache, key)
		}
	}
}
