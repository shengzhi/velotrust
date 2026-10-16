//go:build linux

// SPDX-License-Identifier: BSD-3-Clause

package linux

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"time"

	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"

	"velotrust/gateway/config"
)

// DebugStatus logs live kernel tunnel and peer status without exposing private
// or preshared keys. It is called only when debug logging is enabled.
func (d *Driver) DebugStatus(ctx context.Context) {
	if !slog.Default().Enabled(ctx, slog.LevelDebug) {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()

	switch backend := d.backend.(type) {
	case *kernelBackend:
		client, err := wgctrl.New()
		if err != nil {
			slog.Debug("read gateway WireGuard status failed", "backend", "kernel", "error", err)
			return
		}
		defer client.Close()
		device, err := client.Device(backend.name)
		if err != nil {
			slog.Debug("read gateway WireGuard status failed", "backend", "kernel", "interface", backend.name, "error", err)
			return
		}
		slog.Debug("gateway WireGuard device status", "backend", "kernel", "interface", device.Name, "listen_port", device.ListenPort, "peer_count", len(device.Peers))
		for _, peer := range device.Peers {
			endpoint := ""
			if peer.Endpoint != nil {
				endpoint = peer.Endpoint.String()
			}
			logPeerStatus(keyFingerprint(peer.PublicKey[:]), endpoint, peer.LastHandshakeTime, peer.ReceiveBytes, peer.TransmitBytes)
		}
	case *userspaceBackend:
		// The WireGuard userspace get protocol includes private and preshared
		// keys in its response, so diagnostics deliberately do not query it.
		slog.Debug("gateway WireGuard peer status is not exposed by userspace diagnostics", "backend", "userspace", "interface", d.interfaceName)
	default:
		slog.Debug("gateway WireGuard status unavailable", "interface", d.interfaceName, "reason", "tunnel backend is not initialized")
	}
}

func logTunnelConfiguration(desired config.Tunnel) {
	slog.Debug("gateway WireGuard tunnel configured",
		"interface", desired.Interface,
		"listen_port", desired.ListenPort,
		"address", desired.Address,
		"mtu", desired.EffectiveMTU(),
		"peer_count", len(desired.Peers),
	)
	for _, configuredPeer := range desired.Peers {
		key, err := wgtypes.ParseKey(configuredPeer.PublicKey)
		fingerprint := "invalid"
		if err == nil {
			fingerprint = keyFingerprint(key[:])
		}
		slog.Debug("gateway WireGuard peer configured",
			"peer_id", configuredPeer.ID,
			"public_key_fingerprint", fingerprint,
			"endpoint", configuredPeer.Endpoint,
			"allowed_ips", configuredPeer.AllowedIPs,
			"persistent_keepalive_seconds", configuredPeer.PersistentKeepalive,
		)
	}
}

func logPeerStatus(fingerprint, endpoint string, handshake time.Time, rxBytes, txBytes int64) {
	latestHandshake := "never"
	if !handshake.IsZero() {
		latestHandshake = handshake.UTC().Format(time.RFC3339)
	}
	slog.Debug("gateway WireGuard peer status",
		"public_key_fingerprint", fingerprint,
		"endpoint", endpoint,
		"latest_handshake", latestHandshake,
		"received_bytes", rxBytes,
		"sent_bytes", txBytes,
	)
}

func keyFingerprint(key []byte) string {
	digest := sha256.Sum256(key)
	return hex.EncodeToString(digest[:6])
}
