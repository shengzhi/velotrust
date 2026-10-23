//go:build windows

// SPDX-License-Identifier: BSD-3-Clause

package wireguard

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/netip"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	clientconfig "velotrust/client/internal/config"
	"velotrust/client/internal/dnsproxy"
	winplatform "velotrust/client/system/windows"

	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

type configuredGateway struct {
	id                 string
	peerName           string
	key                wgtypes.Key
	configuredEndpoint string
	endpoint           *net.UDPAddr
	endpointIP         netip.Addr
	allowedIPs         []string
	keepalive          uint16
}

// Connect starts a static client-to-gateway WireGuard session in the current
// process. It returns when the context is canceled and owns all tunnel cleanup.
func Connect(ctx context.Context, instanceID string, configPath string, profile clientconfig.Tunnel, output io.Writer) (resultErr error) {
	return ConnectWithReady(ctx, instanceID, configPath, profile, output, nil)
}

// ConnectWithReady reports the result of tunnel setup through ready before it
// begins waiting for context cancellation. This lets the HUB control loop
// acknowledge a snapshot only after Windows has applied its routes and tunnel.
func ConnectWithReady(ctx context.Context, instanceID string, configPath string, profile clientconfig.Tunnel, output io.Writer, ready chan<- error) (resultErr error) {
	readyReported := false
	reportReady := func(err error) {
		if ready == nil || readyReported {
			return
		}
		ready <- err
		readyReported = true
	}
	defer func() { reportReady(resultErr) }()
	if !winplatform.IsElevated() {
		return errors.New("connect requires an elevated PowerShell session to create the Wintun interface and routes")
	}
	releaseTunnelLock, err := winplatform.AcquireTunnelLock()
	if err != nil {
		return err
	}
	defer func() {
		resultErr = errors.Join(resultErr, releaseTunnelLock())
	}()
	privateKeyPath := filepath.Join(filepath.Dir(configPath), profile.PrivateKeyFile)
	privateKey, err := winplatform.LoadWireGuardPrivateKey(privateKeyPath)
	if err != nil {
		return err
	}
	defer clear(privateKey[:])
	if _, err := fmt.Fprintf(output, "Client WireGuard public key: %s\n", privateKey.PublicKey().String()); err != nil {
		return fmt.Errorf("write client public key: %w", err)
	}
	gateways, routes, err := prepareGateways(profile.ConfiguredPeers())
	if err != nil {
		return err
	}
	var localDNS *dnsproxy.Proxy
	var cleanupDNSCapture func() error
	if profile.DNS != nil && profile.DNS.Enabled {
		if err := winplatform.CleanupDNSCapture(instanceID); err != nil {
			return err
		}
		originalDNS, err := winplatform.DiscoverSystemDNS()
		if err != nil {
			return fmt.Errorf("discover original system DNS: %w", err)
		}
		localDNS, err = dnsproxy.New(*profile.DNS, originalDNS)
		if err != nil {
			return fmt.Errorf("prepare local DNS proxy: %w", err)
		}
		if err := localDNS.Start(); err != nil {
			return fmt.Errorf("start local DNS proxy: %w", err)
		}
		slog.Info("prepared split DNS proxy", "enterprise_resolver_groups", len(profile.DNS.ResolverGroups), "original_resolver_count", len(originalDNS))
		defer func() {
			if cleanupDNSCapture != nil {
				resultErr = errors.Join(resultErr, cleanupDNSCapture())
			}
			resultErr = errors.Join(resultErr, localDNS.Close())
		}()
		dnsMonitorCtx, cancelDNSMonitor := context.WithCancel(ctx)
		defer cancelDNSMonitor()
		startOriginalDNSMonitor(dnsMonitorCtx, localDNS)
	}
	for _, gateway := range gateways {
		slog.Debug("resolved gateway WireGuard endpoint",
			"gateway_id", gateway.id,
			"peer_name", gateway.peerName,
			"configured_endpoint", gateway.configuredEndpoint,
			"resolved_endpoint", gateway.endpoint.String(),
			"peer_key_fingerprint", keyFingerprint(gateway.key),
			"allowed_ips", gateway.allowedIPs,
		)
	}

	interfaceName := profile.InterfaceName
	if interfaceName == "" {
		interfaceName = "VeloTrust"
	}
	mtu := profile.EffectiveMTU()
	slog.Debug("preparing client WireGuard tunnel",
		"interface", interfaceName,
		"address", profile.Address,
		"allowed_ips", routes,
		"gateway_count", len(gateways),
		"mtu", mtu,
	)
	tunnelDevice, err := tun.CreateTUN(interfaceName, mtu)
	if err != nil {
		return fmt.Errorf("create Wintun interface: %w", err)
	}
	slog.Debug("created Wintun interface", "interface", interfaceName)
	var wgDevice *device.Device
	var cleanupNetwork func() error
	defer func() {
		if cleanupNetwork != nil {
			resultErr = errors.Join(resultErr, cleanupNetwork())
		}
		if wgDevice != nil {
			wgDevice.Close()
		} else {
			_ = tunnelDevice.Close()
		}
	}()

	cleanupNetwork, err = winplatform.ConfigureTunnelNetwork(interfaceName, profile.Address, routes)
	if err != nil {
		return err
	}
	slog.Debug("configured client tunnel address and routes", "interface", interfaceName, "address", profile.Address, "routes", routes)
	wgDevice = device.NewDevice(tunnelDevice, conn.NewStdNetBind(), device.NewLogger(device.LogLevelError, "VeloTrust client"))

	var uapi strings.Builder
	fmt.Fprintf(&uapi, "private_key=%s\nreplace_peers=true\n", hex.EncodeToString(privateKey[:]))
	for _, gateway := range gateways {
		fmt.Fprintf(&uapi, "public_key=%s\nendpoint=%s\nreplace_allowed_ips=true\n", hex.EncodeToString(gateway.key[:]), gateway.endpoint.String())
		for _, route := range gateway.allowedIPs {
			fmt.Fprintf(&uapi, "allowed_ip=%s\n", route)
		}
		fmt.Fprintf(&uapi, "persistent_keepalive_interval=%d\n", gateway.keepalive)
	}
	if err := wgDevice.IpcSet(uapi.String()); err != nil {
		uapi.Reset()
		slog.Error("configure client WireGuard peers failed", "gateway_count", len(gateways), "error", err)
		return fmt.Errorf("configure WireGuard peers: %w", err)
	}
	uapi.Reset()
	for _, gateway := range gateways {
		slog.Debug("configured client WireGuard peer",
			"gateway_id", gateway.id,
			"peer_name", gateway.peerName,
			"peer_key_fingerprint", keyFingerprint(gateway.key),
			"endpoint", gateway.endpoint.String(),
			"allowed_ips", gateway.allowedIPs,
			"persistent_keepalive_seconds", gateway.keepalive,
		)
	}
	if err := wgDevice.Up(); err != nil {
		slog.Error("start client WireGuard device failed", "interface", interfaceName, "error", err)
		return fmt.Errorf("start WireGuard tunnel: %w", err)
	}
	if localDNS != nil {
		probeName, err := winplatform.DNSCaptureProbeName()
		if err != nil {
			return err
		}
		cleanupDNSCapture, err = winplatform.ApplyDNSCapture(instanceID, probeName)
		if err != nil {
			return err
		}
		slog.Info("Windows system DNS capture is active", "policy", "suffix-split", "resolver_groups", len(profile.DNS.ResolverGroups))
	}
	slog.Info("client WireGuard device started", "interface", interfaceName, "gateway_count", len(gateways))

	if _, err := fmt.Fprintf(output, "Tunnel interface %q is running; gateways %d; assigned %s; routes %d. Press Ctrl+C to disconnect.\n", interfaceName, len(gateways), profile.Address, len(routes)); err != nil {
		return fmt.Errorf("write connection status: %w", err)
	}
	reportReady(nil)
	return waitForTunnel(ctx, wgDevice, gateways, output, localDNS)
}

func startOriginalDNSMonitor(ctx context.Context, proxy *dnsproxy.Proxy) {
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				upstreams, err := winplatform.DiscoverSystemDNS()
				if err != nil {
					slog.Warn("refresh original DNS upstreams failed", "error", err)
					continue
				}
				proxy.UpdateOriginalDNS(upstreams)
			}
		}
	}()
}

func prepareGateways(peers []clientconfig.Peer) ([]configuredGateway, []string, error) {
	if len(peers) == 0 {
		return nil, nil, errors.New("client tunnel has no configured gateway peers")
	}
	gateways := make([]configuredGateway, 0, len(peers))
	routeSet := make(map[string]struct{})
	routes := make([]string, 0)
	for index, peer := range peers {
		id := peer.ID
		if id == "" {
			id = fmt.Sprintf("gateway-%d", index+1)
		}
		peerKey, err := wgtypes.ParseKey(peer.PublicKey)
		if err != nil {
			return nil, nil, fmt.Errorf("parse WireGuard public key for gateway %q: %w", id, err)
		}
		endpoint, err := net.ResolveUDPAddr("udp", peer.Endpoint)
		if err != nil {
			return nil, nil, fmt.Errorf("resolve gateway %q UDP endpoint %q: %w", id, peer.Endpoint, err)
		}
		if endpoint == nil || endpoint.IP == nil {
			return nil, nil, fmt.Errorf("gateway %q UDP endpoint did not resolve to an IP address", id)
		}
		endpointIP, ok := netip.AddrFromSlice(endpoint.IP)
		if !ok {
			return nil, nil, fmt.Errorf("gateway %q endpoint resolved to an unsupported IP address", id)
		}
		endpointIP = endpointIP.Unmap()
		if endpointIP.IsUnspecified() || endpointIP.IsLoopback() || endpointIP.IsMulticast() || endpointIP.IsLinkLocalUnicast() {
			return nil, nil, fmt.Errorf("gateway %q endpoint must resolve to a unicast non-loopback address", id)
		}
		keepalive := peer.PersistentKeepalive
		if keepalive == 0 {
			keepalive = 25
		}
		gateway := configuredGateway{
			id:                 id,
			peerName:           peer.PeerName,
			key:                peerKey,
			configuredEndpoint: peer.Endpoint,
			endpoint:           endpoint,
			endpointIP:         endpointIP,
			keepalive:          keepalive,
		}
		for _, raw := range peer.AllowedIPs {
			prefix, err := netip.ParsePrefix(raw)
			if err != nil {
				return nil, nil, fmt.Errorf("parse allowed IP %q for gateway %q: %w", raw, id, err)
			}
			prefix = prefix.Masked()
			gateway.allowedIPs = append(gateway.allowedIPs, prefix.String())
			if _, exists := routeSet[prefix.String()]; !exists {
				routeSet[prefix.String()] = struct{}{}
				routes = append(routes, prefix.String())
			}
		}
		gateways = append(gateways, gateway)
	}
	for _, gateway := range gateways {
		for _, route := range routes {
			prefix, _ := netip.ParsePrefix(route)
			if prefix.Contains(gateway.endpointIP) {
				return nil, nil, fmt.Errorf("gateway endpoint %s for %q is covered by tunnel route %s; refusing a recursive tunnel route", gateway.endpoint, gateway.id, prefix)
			}
		}
	}
	sort.Strings(routes)
	return gateways, routes, nil
}

func waitForTunnel(ctx context.Context, wgDevice *device.Device, gateways []configuredGateway, output io.Writer, localDNS *dnsproxy.Proxy) error {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	startedAt := time.Now()
	type peerReportingState struct {
		lastDebugStatus   time.Time
		handshakeReported bool
		pendingReported   bool
	}
	reporting := make([]peerReportingState, len(gateways))
	var dnsDone <-chan struct{}
	if localDNS != nil {
		dnsDone = localDNS.Done()
	}
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-dnsDone:
			if err := localDNS.Err(); err != nil {
				return fmt.Errorf("local DNS proxy stopped: %w", err)
			}
			return errors.New("local DNS proxy stopped unexpectedly")
		case <-ticker.C:
			state, err := wgDevice.IpcGet()
			if err != nil {
				slog.Error("read client WireGuard peer status failed", "error", err)
				return fmt.Errorf("read WireGuard peer status: %w", err)
			}
			for i, gateway := range gateways {
				peerStatus := inspectPeerStatus(state, gateway.key)
				if !peerStatus.lastHandshake.IsZero() && !reporting[i].handshakeReported {
					reporting[i].handshakeReported = true
					slog.Info("client WireGuard handshake established",
						"gateway_id", gateway.id,
						"peer_key_fingerprint", keyFingerprint(gateway.key),
						"endpoint", gateway.endpoint.String(),
						"last_handshake", peerStatus.lastHandshake.UTC().Format(time.RFC3339),
					)
					if _, err := fmt.Fprintf(output, "WireGuard handshake established with gateway %q.\n", gateway.id); err != nil {
						return fmt.Errorf("write handshake status for gateway %q: %w", gateway.id, err)
					}
				}
				if peerStatus.lastHandshake.IsZero() && !reporting[i].pendingReported && time.Since(startedAt) >= 10*time.Second {
					reporting[i].pendingReported = true
					slog.Warn("client WireGuard handshake is still pending",
						"gateway_id", gateway.id,
						"peer_key_fingerprint", keyFingerprint(gateway.key),
						"endpoint", gateway.endpoint.String(),
						"peer_found", peerStatus.found,
					)
					if _, err := fmt.Fprintf(output, "No handshake with gateway %q yet. Check its UDP endpoint, peer key, and configuration.\n", gateway.id); err != nil {
						return fmt.Errorf("write pending handshake status for gateway %q: %w", gateway.id, err)
					}
				}
				if reporting[i].lastDebugStatus.IsZero() || time.Since(reporting[i].lastDebugStatus) >= 10*time.Second {
					reporting[i].lastDebugStatus = time.Now()
					latestHandshake := "never"
					if !peerStatus.lastHandshake.IsZero() {
						latestHandshake = peerStatus.lastHandshake.UTC().Format(time.RFC3339)
					}
					slog.Debug("client WireGuard peer status",
						"gateway_id", gateway.id,
						"peer_key_fingerprint", keyFingerprint(gateway.key),
						"endpoint", gateway.endpoint.String(),
						"peer_found", peerStatus.found,
						"latest_handshake", latestHandshake,
						"received_bytes", peerStatus.rxBytes,
						"sent_bytes", peerStatus.txBytes,
					)
				}
			}
		}
	}
}

type wireGuardPeerStatus struct {
	found         bool
	lastHandshake time.Time
	rxBytes       int64
	txBytes       int64
}

func inspectPeerStatus(state string, peerKey wgtypes.Key) wireGuardPeerStatus {
	needle := "public_key=" + hex.EncodeToString(peerKey[:])
	status := wireGuardPeerStatus{}
	inPeer := false
	for _, line := range strings.Split(state, "\n") {
		if strings.HasPrefix(line, "public_key=") {
			inPeer = line == needle
			if inPeer {
				status.found = true
			}
			continue
		}
		if !inPeer {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		switch key {
		case "last_handshake_time_sec":
			seconds, err := strconv.ParseInt(value, 10, 64)
			if err == nil && seconds > 0 {
				status.lastHandshake = time.Unix(seconds, 0).UTC()
			}
		case "rx_bytes":
			status.rxBytes, _ = strconv.ParseInt(value, 10, 64)
		case "tx_bytes":
			status.txBytes, _ = strconv.ParseInt(value, 10, 64)
		}
	}
	return status
}

func keyFingerprint(key wgtypes.Key) string {
	digest := sha256.Sum256(key[:])
	return hex.EncodeToString(digest[:6])
}
