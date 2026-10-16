//go:build linux

package linux

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/vishvananda/netlink"
	"golang.zx2c4.com/wireguard/conn"
	wgdevice "golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun"
	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"

	"velotrust/gateway/config"
)

const (
	filterTableName      = "velotrust_gateway_filter"
	natTableName         = "velotrust_gateway_nat"
	forwardChainName     = "gateway_forward"
	preroutingChainName  = "gateway_prerouting"
	postroutingChainName = "gateway_postrouting"
	managedRouteProto    = 253
	defaultMTU           = 1420
)

type tunnelBackend interface {
	configure(context.Context, config.Tunnel) error
	close() error
}

// Driver manages the configured tunnel, its scoped nftables rules, and its
// optional Docker DOCKER-USER integration. Unrelated host forwarding policy
// is left intact.
type Driver struct {
	mu sync.Mutex

	interfaceName string
	backend       tunnelBackend
	blocked       bool
	rules         []config.ACLRule
	needsIPv4     bool
	needsIPv6     bool
	forwarding    map[string]string
}

func New(interfaceName string) *Driver {
	return &Driver{interfaceName: interfaceName, blocked: true}
}

func (d *Driver) BlockForwarding(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.blocked = true
	return d.replaceFirewallLocked()
}

func (d *Driver) ApplyTunnel(ctx context.Context, desired config.Tunnel) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if desired.Interface != d.interfaceName {
		return errors.New("tunnel interface differs from the managed interface")
	}
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.backend == nil {
		backend, err := createBackend(ctx, desired)
		if err != nil {
			return fmt.Errorf("initialize WireGuard tunnel: %w", err)
		}
		d.backend = backend
	} else if err := d.backend.configure(ctx, desired); err != nil {
		return fmt.Errorf("configure WireGuard tunnel: %w", err)
	}

	link, err := netlink.LinkByName(d.interfaceName)
	if err != nil {
		return fmt.Errorf("find tunnel link: %w", err)
	}
	if err := configureAddress(link, desired.Address); err != nil {
		return fmt.Errorf("configure tunnel address: %w", err)
	}
	if err := netlink.LinkSetMTU(link, defaultMTU); err != nil {
		return fmt.Errorf("set tunnel MTU: %w", err)
	}
	if err := netlink.LinkSetUp(link); err != nil {
		return fmt.Errorf("bring tunnel interface up: %w", err)
	}
	logTunnelConfiguration(desired)
	slog.Debug("gateway tunnel interface is up", "interface", d.interfaceName, "address", desired.Address, "mtu", defaultMTU)
	d.needsIPv4, d.needsIPv6 = tunnelFamilies(desired)
	return nil
}

func (d *Driver) ApplyFirewall(ctx context.Context, rules []config.ACLRule) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.rules = append([]config.ACLRule(nil), rules...)
	for _, rule := range d.rules {
		if prefix, err := netip.ParsePrefix(rule.SourcePrefix); err == nil {
			if prefix.Addr().Is4() {
				d.needsIPv4 = true
			} else {
				d.needsIPv6 = true
			}
		}
	}
	if err := d.replaceFirewallLocked(); err != nil {
		return fmt.Errorf("replace gateway firewall rules: %w", err)
	}
	return nil
}

func (d *Driver) ApplyRoutes(ctx context.Context, prefixes []string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	link, err := netlink.LinkByName(d.interfaceName)
	if err != nil {
		return fmt.Errorf("find managed tunnel for routes: %w", err)
	}

	owned, err := netlink.RouteListFiltered(netlink.FAMILY_ALL, &netlink.Route{
		LinkIndex: link.Attrs().Index,
		Protocol:  netlink.RouteProtocol(managedRouteProto),
	}, netlink.RT_FILTER_OIF|netlink.RT_FILTER_PROTOCOL)
	if err != nil {
		return fmt.Errorf("list previously managed routes: %w", err)
	}
	for i := range owned {
		if err := netlink.RouteDel(&owned[i]); err != nil && !errors.Is(err, syscall.ESRCH) {
			return fmt.Errorf("remove previous managed route: %w", err)
		}
	}
	for _, raw := range prefixes {
		prefix, err := netip.ParsePrefix(raw)
		if err != nil || !prefix.IsValid() || prefix.Addr().Is4In6() {
			return fmt.Errorf("invalid route prefix %q", raw)
		}
		prefix = prefix.Masked()
		dst := &net.IPNet{IP: net.IP(prefix.Addr().AsSlice()), Mask: net.CIDRMask(prefix.Bits(), prefix.Addr().BitLen())}
		route := &netlink.Route{
			LinkIndex: link.Attrs().Index,
			Dst:       dst,
			Scope:     netlink.SCOPE_UNIVERSE,
			Protocol:  netlink.RouteProtocol(managedRouteProto),
		}
		if err := netlink.RouteAdd(route); err != nil && !errors.Is(err, syscall.EEXIST) {
			return fmt.Errorf("add route %s: %w", prefix, err)
		}
		if prefix.Addr().Is4() {
			d.needsIPv4 = true
		} else {
			d.needsIPv6 = true
		}
	}
	return nil
}

func (d *Driver) ApplyNAT(ctx context.Context, mappings []config.NATMapping) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if err := validateSNATAddresses(mappings); err != nil {
		return fmt.Errorf("validate gateway SNAT addresses: %w", err)
	}
	if err := d.replaceNATLocked(mappings); err != nil {
		return fmt.Errorf("replace gateway NAT mappings: %w", err)
	}
	for _, mapping := range mappings {
		if mapping.Kind() == config.NATTypeSNAT {
			slog.Debug("gateway SNAT mapping configured",
				"source_prefix", mapping.SourcePrefix,
				"destination_prefix", mapping.DestinationPrefix,
				"egress_interface", mapping.EgressInterface,
				"to_address", mapping.ToAddress,
			)
		} else {
			slog.Debug("gateway DNAT mapping configured",
				"shadow_prefix", mapping.ShadowPrefix,
				"real_prefix", mapping.RealPrefix,
			)
		}
		prefixText := mapping.ShadowPrefix
		if mapping.Kind() == config.NATTypeSNAT {
			prefixText = mapping.SourcePrefix
		}
		prefix, err := netip.ParsePrefix(prefixText)
		if err != nil {
			return fmt.Errorf("invalid NAT source prefix %q: %w", prefixText, err)
		}
		if prefix.Addr().Is4() {
			d.needsIPv4 = true
		} else {
			d.needsIPv6 = true
		}
	}
	return nil
}

func validateSNATAddresses(mappings []config.NATMapping) error {
	for index, mapping := range mappings {
		if mapping.Kind() != config.NATTypeSNAT {
			continue
		}
		wanted, err := netip.ParseAddr(mapping.ToAddress)
		if err != nil {
			return fmt.Errorf("nat[%d].to_address is invalid: %w", index, err)
		}
		link, err := netlink.LinkByName(mapping.EgressInterface)
		if err != nil {
			return fmt.Errorf("find SNAT egress interface %q: %w", mapping.EgressInterface, err)
		}
		family := netlink.FAMILY_V6
		if wanted.Is4() {
			family = netlink.FAMILY_V4
		}
		addresses, err := netlink.AddrList(link, family)
		if err != nil {
			return fmt.Errorf("list addresses on SNAT egress interface %q: %w", mapping.EgressInterface, err)
		}
		found := false
		for _, address := range addresses {
			if address.IPNet == nil {
				continue
			}
			actual, ok := netip.AddrFromSlice(address.IPNet.IP)
			if ok && actual.Unmap() == wanted {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("nat[%d].to_address %s is not assigned to egress interface %q", index, wanted, mapping.EgressInterface)
		}
	}
	return nil
}

func (d *Driver) EnableForwarding(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.forwarding == nil {
		d.forwarding = make(map[string]string)
	}
	if d.needsIPv4 {
		if err := setForwarding("/proc/sys/net/ipv4/ip_forward", d.forwarding); err != nil {
			return err
		}
	}
	if d.needsIPv6 {
		if err := setForwarding("/proc/sys/net/ipv6/conf/all/forwarding", d.forwarding); err != nil {
			return err
		}
	}
	return nil
}

func (d *Driver) UnblockForwarding(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.blocked = false
	if err := d.replaceFirewallLocked(); err != nil {
		d.blocked = true
		blockErr := d.replaceFirewallLocked()
		if blockErr != nil {
			return errors.Join(fmt.Errorf("remove gateway forwarding block: %w", err), fmt.Errorf("restore fail-closed forwarding block: %w", blockErr))
		}
		return fmt.Errorf("remove gateway forwarding block: %w", err)
	}
	return nil
}

func (d *Driver) Close(ctx context.Context) error {
	if ctx == nil {
		return errors.New("context is required")
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.blocked = true
	var result error
	if err := d.replaceFirewallLocked(); err != nil {
		result = errors.Join(result, fmt.Errorf("install gateway block during cleanup: %w", err))
	}
	if err := d.removeDockerForwardRulesLocked(); err != nil {
		result = errors.Join(result, fmt.Errorf("remove Docker forwarding integration: %w", err))
	}
	if d.backend != nil {
		result = errors.Join(result, d.backend.close())
		d.backend = nil
	}
	link, linkErr := netlink.LinkByName(d.interfaceName)
	if linkErr == nil {
		owned, err := netlink.RouteListFiltered(netlink.FAMILY_ALL, &netlink.Route{
			LinkIndex: link.Attrs().Index,
			Protocol:  netlink.RouteProtocol(managedRouteProto),
		}, netlink.RT_FILTER_OIF|netlink.RT_FILTER_PROTOCOL)
		if err != nil {
			result = errors.Join(result, fmt.Errorf("list gateway routes for cleanup: %w", err))
		} else {
			for i := range owned {
				if err := netlink.RouteDel(&owned[i]); err != nil && !errors.Is(err, syscall.ESRCH) {
					result = errors.Join(result, fmt.Errorf("remove gateway route: %w", err))
				}
			}
		}
		if err := netlink.LinkDel(link); err != nil && !errors.Is(err, syscall.ENODEV) {
			result = errors.Join(result, fmt.Errorf("remove managed tunnel interface: %w", err))
		}
	} else if !isLinkNotFound(linkErr) {
		result = errors.Join(result, fmt.Errorf("find tunnel during cleanup: %w", linkErr))
	}
	result = errors.Join(result, restoreForwarding(d.forwarding))
	d.forwarding = nil
	return result
}

func createBackend(ctx context.Context, desired config.Tunnel) (tunnelBackend, error) {
	link, err := netlink.LinkByName(desired.Interface)
	created := false
	if err == nil {
		if link.Type() != "wireguard" {
			return nil, fmt.Errorf("interface %q already exists with type %q", desired.Interface, link.Type())
		}
	} else if !isLinkNotFound(err) {
		return nil, fmt.Errorf("inspect tunnel interface: %w", err)
	} else {
		if err := netlink.LinkAdd(&netlink.Wireguard{LinkAttrs: netlink.LinkAttrs{Name: desired.Interface}}); err == nil {
			created = true
		} else if !isKernelFeatureUnavailable(err) {
			return nil, fmt.Errorf("create kernel WireGuard interface: %w", err)
		} else {
			slog.Debug("kernel WireGuard is unavailable; using userspace backend", "interface", desired.Interface, "reason", err)
			return createUserspaceBackend(ctx, desired)
		}
	}

	err = configureKernel(ctx, desired)
	if err == nil {
		slog.Debug("using kernel WireGuard backend", "interface", desired.Interface, "listen_port", desired.ListenPort)
		return &kernelBackend{name: desired.Interface}, nil
	}
	if isKernelFeatureUnavailable(err) && created {
		if link, findErr := netlink.LinkByName(desired.Interface); findErr == nil {
			if delErr := netlink.LinkDel(link); delErr != nil {
				return nil, errors.Join(fmt.Errorf("kernel WireGuard unavailable: %w", err), fmt.Errorf("remove partial kernel interface: %w", delErr))
			}
		}
		slog.Debug("kernel WireGuard is unavailable; using userspace backend", "interface", desired.Interface, "reason", err)
		return createUserspaceBackend(ctx, desired)
	}
	if created {
		if link, findErr := netlink.LinkByName(desired.Interface); findErr == nil {
			_ = netlink.LinkDel(link)
		}
	}
	return nil, fmt.Errorf("configure kernel WireGuard: %w", err)
}

type kernelBackend struct{ name string }

func (*kernelBackend) configure(ctx context.Context, desired config.Tunnel) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return configureKernel(ctx, desired)
}

func (*kernelBackend) close() error { return nil }

func configureKernel(ctx context.Context, desired config.Tunnel) error {
	privateKey, err := wgtypes.ParseKey(desired.PrivateKey)
	if err != nil {
		return fmt.Errorf("parse WireGuard private key: %w", err)
	}
	peers, err := kernelPeers(ctx, desired.Peers)
	if err != nil {
		return err
	}
	client, err := wgctrl.New()
	if err != nil {
		return err
	}
	defer client.Close()
	port := int(desired.ListenPort)
	return client.ConfigureDevice(desired.Interface, wgtypes.Config{
		PrivateKey:   &privateKey,
		ListenPort:   &port,
		ReplacePeers: true,
		Peers:        peers,
	})
}

func kernelPeers(ctx context.Context, peers []config.Peer) ([]wgtypes.PeerConfig, error) {
	result := make([]wgtypes.PeerConfig, 0, len(peers))
	for _, peer := range peers {
		key, err := wgtypes.ParseKey(peer.PublicKey)
		if err != nil {
			return nil, fmt.Errorf("parse peer %q key: %w", peer.ID, err)
		}
		allowed := make([]net.IPNet, 0, len(peer.AllowedIPs))
		for _, raw := range peer.AllowedIPs {
			_, prefix, err := net.ParseCIDR(raw)
			if err != nil {
				return nil, fmt.Errorf("parse peer %q allowed prefix %q: %w", peer.ID, raw, err)
			}
			allowed = append(allowed, *prefix)
		}
		var endpoint *net.UDPAddr
		if peer.Endpoint != "" {
			endpoint, err = resolveEndpoint(ctx, peer.Endpoint)
			if err != nil {
				return nil, fmt.Errorf("resolve peer %q endpoint: %w", peer.ID, err)
			}
		}
		keepalive := timeDurationSeconds(peer.PersistentKeepalive)
		var keepalivePtr *time.Duration
		if keepalive != 0 {
			keepalivePtr = &keepalive
		}
		result = append(result, wgtypes.PeerConfig{
			PublicKey:                   key,
			Endpoint:                    endpoint,
			PersistentKeepaliveInterval: keepalivePtr,
			ReplaceAllowedIPs:           true,
			AllowedIPs:                  allowed,
		})
	}
	return result, nil
}

func resolveEndpoint(ctx context.Context, endpoint string) (*net.UDPAddr, error) {
	host, portText, err := net.SplitHostPort(endpoint)
	if err != nil {
		return nil, err
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		return nil, err
	}
	if address, err := netip.ParseAddr(host); err == nil {
		return &net.UDPAddr{IP: net.IP(address.AsSlice()), Port: port, Zone: address.Zone()}, nil
	}
	addresses, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return nil, err
	}
	if len(addresses) == 0 {
		return nil, errors.New("endpoint resolved to no IP addresses")
	}
	return &net.UDPAddr{IP: net.IP(addresses[0].AsSlice()), Port: port, Zone: addresses[0].Zone()}, nil
}

func timeDurationSeconds(seconds uint16) time.Duration {
	return time.Duration(seconds) * time.Second
}

type userspaceBackend struct {
	device *wgdevice.Device
	tun    tun.Device
}

func createUserspaceBackend(ctx context.Context, desired config.Tunnel) (tunnelBackend, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	tunDevice, err := tun.CreateTUN(desired.Interface, defaultMTU)
	if err != nil {
		return nil, fmt.Errorf("create userspace TUN interface: %w", err)
	}
	device := wgdevice.NewDevice(tunDevice, conn.NewDefaultBind(), wgdevice.NewLogger(wgdevice.LogLevelSilent, ""))
	backend := &userspaceBackend{device: device, tun: tunDevice}
	if err := backend.configure(ctx, desired); err != nil {
		_ = backend.close()
		return nil, err
	}
	return backend, nil
}

func (b *userspaceBackend) configure(ctx context.Context, desired config.Tunnel) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	privateKey, err := base64.StdEncoding.DecodeString(desired.PrivateKey)
	if err != nil {
		return err
	}
	var builder strings.Builder
	fmt.Fprintf(&builder, "private_key=%s\nlisten_port=%d\nreplace_peers=true\n", hex.EncodeToString(privateKey), desired.ListenPort)
	for _, peer := range desired.Peers {
		key, err := base64.StdEncoding.DecodeString(peer.PublicKey)
		if err != nil {
			return fmt.Errorf("decode peer %q key: %w", peer.ID, err)
		}
		fmt.Fprintf(&builder, "public_key=%s\nreplace_allowed_ips=true\n", hex.EncodeToString(key))
		if peer.Endpoint != "" {
			fmt.Fprintf(&builder, "endpoint=%s\n", peer.Endpoint)
		}
		if peer.PersistentKeepalive != 0 {
			fmt.Fprintf(&builder, "persistent_keepalive_interval=%d\n", peer.PersistentKeepalive)
		}
		for _, prefix := range peer.AllowedIPs {
			fmt.Fprintf(&builder, "allowed_ip=%s\n", prefix)
		}
	}
	if err := b.device.IpcSet(builder.String()); err != nil {
		return fmt.Errorf("configure userspace WireGuard device: %w", err)
	}
	if err := b.device.Up(); err != nil {
		return fmt.Errorf("start userspace WireGuard device: %w", err)
	}
	return nil
}

func (b *userspaceBackend) close() error {
	if b.device != nil {
		b.device.Close()
		b.device = nil
	}
	if b.tun != nil {
		err := b.tun.Close()
		b.tun = nil
		return err
	}
	return nil
}

func isKernelFeatureUnavailable(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, syscall.EOPNOTSUPP) || errors.Is(err, syscall.ENOTSUP) || errors.Is(err, syscall.ENODEV) || errors.Is(err, os.ErrNotExist) {
		return true
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "operation not supported") || strings.Contains(message, "protocol not supported") || strings.Contains(message, "wireguard is not supported") || strings.Contains(message, "unknown device type")
}

func isLinkNotFound(err error) bool {
	if err == nil {
		return false
	}
	var notFound netlink.LinkNotFoundError
	return errors.As(err, &notFound) || errors.Is(err, syscall.ENODEV)
}

func configureAddress(link netlink.Link, raw string) error {
	address, err := netlink.ParseAddr(raw)
	if err != nil {
		return err
	}
	addresses, err := netlink.AddrList(link, netlink.FAMILY_ALL)
	if err != nil {
		return err
	}
	for i := range addresses {
		if addresses[i].IPNet == nil || addresses[i].IPNet.String() == address.IPNet.String() || addresses[i].IP.IsLinkLocalUnicast() {
			continue
		}
		if err := netlink.AddrDel(link, &addresses[i]); err != nil && !errors.Is(err, syscall.EADDRNOTAVAIL) {
			return err
		}
	}
	return netlink.AddrReplace(link, address)
}

func tunnelFamilies(tunnel config.Tunnel) (bool, bool) {
	v4, v6 := false, false
	add := func(raw string) {
		prefix, err := netip.ParsePrefix(raw)
		if err != nil {
			return
		}
		if prefix.Addr().Is4() {
			v4 = true
		} else {
			v6 = true
		}
	}
	add(tunnel.Address)
	for _, peer := range tunnel.Peers {
		for _, prefix := range peer.AllowedIPs {
			add(prefix)
		}
	}
	return v4, v6
}

func setForwarding(path string, before map[string]string) error {
	if _, exists := before[path]; exists {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read forwarding setting %s: %w", path, err)
	}
	value := strings.TrimSpace(string(data))
	before[path] = value
	if value == "1" {
		return nil
	}
	if err := os.WriteFile(path, []byte("1\n"), 0o644); err != nil {
		delete(before, path)
		return fmt.Errorf("enable forwarding in %s: %w", path, err)
	}
	return nil
}

func restoreForwarding(before map[string]string) error {
	var result error
	for path, old := range before {
		if old == "1" {
			continue
		}
		current, err := os.ReadFile(path)
		if err != nil {
			result = errors.Join(result, fmt.Errorf("read forwarding setting %s during restore: %w", path, err))
			continue
		}
		if strings.TrimSpace(string(current)) != "1" {
			continue
		}
		if err := os.WriteFile(path, []byte(old+"\n"), 0o644); err != nil {
			result = errors.Join(result, fmt.Errorf("restore forwarding setting %s: %w", path, err))
		}
	}
	return result
}
