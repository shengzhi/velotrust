// SPDX-License-Identifier: BSD-3-Clause

// Package config contains the local, non-secret client configuration.
package config

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const MaxFileSize = 1 << 20

var instanceIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)
var peerIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

const maxTunnelPeers = 64

// Local describes how this installation identifies its private control-plane
// instance. It must never contain user credentials, device keys, or tokens.
type Local struct {
	SchemaVersion   int     `json:"schema_version"`
	InstanceID      string  `json:"instance_id"`
	ControlPlaneURL string  `json:"control_plane_url"`
	LogLevel        string  `json:"log_level,omitempty"`
	Tunnel          *Tunnel `json:"tunnel,omitempty"`
}

// Tunnel is a static, locally provisioned WireGuard client profile. It does
// not represent an authorization decision; AllowedIPs must be provisioned by
// an administrator and the gateway must enforce its own default-deny policy.
type Tunnel struct {
	InterfaceName  string `json:"interface_name,omitempty"`
	Address        string `json:"address"`
	PrivateKeyFile string `json:"private_key_file"`
	MTU            int    `json:"mtu,omitempty"`
	Peer           *Peer  `json:"peer,omitempty"`
	Peers          []Peer `json:"peers,omitempty"`
}

type Peer struct {
	ID                  string   `json:"id,omitempty"`
	PublicKey           string   `json:"public_key"`
	Endpoint            string   `json:"endpoint"`
	AllowedIPs          []string `json:"allowed_ips"`
	PersistentKeepalive uint16   `json:"persistent_keepalive,omitempty"`
}

// ConfiguredPeers returns the configured gateway peers. Peer is retained as
// the legacy single-gateway field; new configurations should use Peers.
func (t Tunnel) ConfiguredPeers() []Peer {
	if len(t.Peers) != 0 {
		return t.Peers
	}
	if t.Peer != nil {
		return []Peer{*t.Peer}
	}
	return nil
}

// Load reads and validates a local client configuration file.
func Load(path string) (Local, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return Local{}, fmt.Errorf("inspect client config: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return Local{}, errors.New("client config must be a regular non-symlink file")
	}
	if info.Size() > MaxFileSize {
		return Local{}, fmt.Errorf("client config exceeds %d bytes", MaxFileSize)
	}

	file, err := os.Open(path)
	if err != nil {
		return Local{}, fmt.Errorf("open client config: %w", err)
	}
	defer file.Close()
	openedInfo, err := file.Stat()
	if err != nil {
		return Local{}, fmt.Errorf("inspect opened client config: %w", err)
	}
	currentInfo, err := os.Lstat(path)
	if err != nil {
		return Local{}, fmt.Errorf("reinspect client config: %w", err)
	}
	if currentInfo.Mode()&os.ModeSymlink != 0 || !currentInfo.Mode().IsRegular() || !os.SameFile(info, openedInfo) || !os.SameFile(currentInfo, openedInfo) {
		return Local{}, errors.New("client config changed while opening")
	}

	data, err := io.ReadAll(io.LimitReader(file, MaxFileSize+1))
	if err != nil {
		return Local{}, fmt.Errorf("read client config: %w", err)
	}
	if len(data) > MaxFileSize {
		return Local{}, fmt.Errorf("client config exceeds %d bytes", MaxFileSize)
	}
	if err := rejectDuplicateFields(data); err != nil {
		return Local{}, err
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var cfg Local
	if err := decoder.Decode(&cfg); err != nil {
		return Local{}, fmt.Errorf("decode client config: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			return Local{}, errors.New("client config contains multiple JSON values")
		}
		return Local{}, fmt.Errorf("decode trailing client config data: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return Local{}, err
	}
	return cfg, nil
}

func rejectDuplicateFields(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := scanValue(decoder); err != nil {
		return fmt.Errorf("decode client config: %w", err)
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("client config contains multiple JSON values")
		}
		return fmt.Errorf("decode trailing client config data: %w", err)
	}
	return nil
}

func scanValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		seen := make(map[string]struct{})
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok {
				return errors.New("client config field name must be a string")
			}
			if _, exists := seen[key]; exists {
				return fmt.Errorf("client config contains duplicate field %q", key)
			}
			seen[key] = struct{}{}
			if err := scanValue(decoder); err != nil {
				return err
			}
		}
		_, err = decoder.Token()
		return err
	case '[':
		for decoder.More() {
			if err := scanValue(decoder); err != nil {
				return err
			}
		}
		_, err = decoder.Token()
		return err
	default:
		return fmt.Errorf("unexpected JSON delimiter %q", delim)
	}
}

// Validate checks fields used to bind the client to its enterprise instance.
func (cfg Local) Validate() error {
	if cfg.SchemaVersion != 1 {
		return fmt.Errorf("unsupported client config schema_version %d", cfg.SchemaVersion)
	}
	if !instanceIDPattern.MatchString(cfg.InstanceID) {
		return errors.New("instance_id must be 1 to 128 safe ASCII characters")
	}
	if cfg.ControlPlaneURL == "" {
		if cfg.Tunnel == nil {
			return errors.New("control_plane_url is required unless a static tunnel profile is configured")
		}
	} else {
		if strings.TrimSpace(cfg.ControlPlaneURL) != cfg.ControlPlaneURL {
			return errors.New("control_plane_url must not contain surrounding whitespace")
		}
		parsed, err := url.Parse(cfg.ControlPlaneURL)
		if err != nil {
			return fmt.Errorf("parse control_plane_url: %w", err)
		}
		if !strings.EqualFold(parsed.Scheme, "https") || parsed.Hostname() == "" || parsed.User != nil || parsed.Opaque != "" || parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" {
			return errors.New("control_plane_url must be an HTTPS URL without credentials, query, or fragment")
		}
		if strings.HasSuffix(parsed.Host, ":") {
			return errors.New("control_plane_url must not have an empty port")
		}
		if port := parsed.Port(); port != "" {
			value, err := strconv.ParseUint(port, 10, 16)
			if err != nil || value == 0 {
				return errors.New("control_plane_url port must be between 1 and 65535")
			}
		}
	}
	if cfg.LogLevel != "" && cfg.LogLevel != "debug" && cfg.LogLevel != "info" && cfg.LogLevel != "warn" && cfg.LogLevel != "error" {
		return fmt.Errorf("log_level must be one of debug, info, warn, or error")
	}
	if cfg.Tunnel != nil {
		if err := cfg.Tunnel.Validate(); err != nil {
			return fmt.Errorf("tunnel: %w", err)
		}
	}
	return nil
}

func (t Tunnel) Validate() error {
	if t.InterfaceName != "" && t.InterfaceName != "VeloTrust" {
		return errors.New("interface_name is reserved and must be VeloTrust")
	}
	if t.PrivateKeyFile == "" || filepath.IsAbs(t.PrivateKeyFile) || filepath.Base(t.PrivateKeyFile) != t.PrivateKeyFile || strings.ContainsAny(t.PrivateKeyFile, ":/\\") || strings.TrimSpace(t.PrivateKeyFile) != t.PrivateKeyFile {
		return errors.New("private_key_file must be a relative filename inside the config directory")
	}
	address, err := netip.ParsePrefix(t.Address)
	if err != nil || !address.IsValid() || address.Bits() == 0 || address.Addr().IsUnspecified() || address.Addr().IsMulticast() || address.Addr().Is4In6() {
		return errors.New("address must be a valid unicast IP prefix")
	}
	if t.MTU != 0 && (t.MTU < 1280 || t.MTU > 9000) {
		return errors.New("mtu must be between 1280 and 9000")
	}
	if t.Peer != nil && len(t.Peers) != 0 {
		return errors.New("configure either tunnel.peer or tunnel.peers, not both")
	}
	peers := t.ConfiguredPeers()
	if len(peers) == 0 || len(peers) > maxTunnelPeers {
		return fmt.Errorf("tunnel must configure between 1 and %d gateway peers", maxTunnelPeers)
	}
	seenIDs := make(map[string]struct{}, len(peers))
	seenKeys := make(map[string]struct{}, len(peers))
	seenAllowedIPs := make(map[netip.Prefix]int)
	for peerIndex, peer := range peers {
		field := "peers[" + strconv.Itoa(peerIndex) + "]"
		if t.Peer != nil {
			field = "peer"
		}
		if peer.ID != "" {
			if !peerIDPattern.MatchString(peer.ID) {
				return fmt.Errorf("%s.id must be 1 to 64 safe ASCII characters", field)
			}
			if _, exists := seenIDs[peer.ID]; exists {
				return fmt.Errorf("%s.id is duplicated", field)
			}
			seenIDs[peer.ID] = struct{}{}
		}
		if err := validateKey(peer.PublicKey); err != nil {
			return fmt.Errorf("%s.public_key: %w", field, err)
		}
		if _, exists := seenKeys[peer.PublicKey]; exists {
			return fmt.Errorf("%s.public_key duplicates another gateway peer", field)
		}
		seenKeys[peer.PublicKey] = struct{}{}
		host, port, endpointErr := net.SplitHostPort(peer.Endpoint)
		if endpointErr != nil || host == "" || strings.TrimSpace(peer.Endpoint) != peer.Endpoint || strings.ContainsAny(peer.Endpoint, "\r\n\t /\\") {
			return fmt.Errorf("%s.endpoint must be a host:port UDP endpoint", field)
		}
		portValue, err := strconv.ParseUint(port, 10, 16)
		if err != nil || portValue == 0 {
			return fmt.Errorf("%s.endpoint port must be between 1 and 65535", field)
		}
		if len(peer.AllowedIPs) == 0 || len(peer.AllowedIPs) > 256 {
			return fmt.Errorf("%s.allowed_ips must contain between 1 and 256 prefixes", field)
		}
		seenOnPeer := make(map[netip.Prefix]struct{}, len(peer.AllowedIPs))
		for i, raw := range peer.AllowedIPs {
			prefix, err := netip.ParsePrefix(raw)
			if err != nil || !prefix.IsValid() || prefix.Bits() == 0 || prefix.Addr().IsUnspecified() || prefix.Addr().IsMulticast() || prefix.Addr().Is4In6() {
				return fmt.Errorf("%s.allowed_ips[%d] must be a valid unicast network prefix and cannot be a default route", field, i)
			}
			prefix = prefix.Masked()
			if _, exists := seenOnPeer[prefix]; exists {
				return fmt.Errorf("%s.allowed_ips[%d] is duplicated", field, i)
			}
			seenOnPeer[prefix] = struct{}{}
			if previousPeer, exists := seenAllowedIPs[prefix]; exists {
				return fmt.Errorf("%s.allowed_ips[%d] duplicates the same prefix on peers[%d]; assign each exact prefix to only one gateway", field, i, previousPeer)
			}
			seenAllowedIPs[prefix] = peerIndex
		}
	}
	return nil
}

func validateKey(value string) error {
	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil || len(decoded) != 32 || base64.StdEncoding.EncodeToString(decoded) != value {
		return errors.New("must be canonical base64 encoding of a 32-byte WireGuard key")
	}
	allZero := true
	for _, b := range decoded {
		allZero = allZero && b == 0
	}
	if allZero {
		return errors.New("must not be an all-zero WireGuard key")
	}
	return nil
}

// EffectiveLogLevel returns the configured level or the safe default.
func (cfg Local) EffectiveLogLevel() string {
	if cfg.LogLevel == "" {
		return "info"
	}
	return cfg.LogLevel
}
