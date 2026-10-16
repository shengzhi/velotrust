package config

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const maxConfigBytes = 2 << 20

// DefaultTunnelMTU preserves the existing Gateway MTU when the field is omitted.
const DefaultTunnelMTU = 1420

var (
	identifierPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)
	interfacePattern  = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,15}$`)
)

// Config is the complete desired data-plane state for one gateway generation.
// An empty rule list intentionally means deny all forwarded traffic.
type Config struct {
	SchemaVersion     uint32       `json:"schema_version"`
	GatewayID         string       `json:"gateway_id"`
	Generation        uint64       `json:"generation"`
	AllowDefaultRoute bool         `json:"allow_default_route,omitempty"`
	Tunnel            Tunnel       `json:"tunnel"`
	Routes            []string     `json:"routes,omitempty"`
	NAT               []NATMapping `json:"nat,omitempty"`
	Rules             []ACLRule    `json:"rules,omitempty"`
}

type Tunnel struct {
	Interface      string `json:"interface"`
	Address        string `json:"address"`
	ListenPort     uint16 `json:"listen_port"`
	MTU            int    `json:"mtu,omitempty"`
	PrivateKey     string `json:"private_key,omitempty"`
	PrivateKeyFile string `json:"private_key_file,omitempty"`
	Peers          []Peer `json:"peers,omitempty"`
}

// EffectiveMTU returns the configured MTU or the backwards-compatible default.
func (t Tunnel) EffectiveMTU() int {
	if t.MTU == 0 {
		return DefaultTunnelMTU
	}
	return t.MTU
}

type Peer struct {
	ID                  string   `json:"id"`
	PublicKey           string   `json:"public_key"`
	Endpoint            string   `json:"endpoint,omitempty"`
	AllowedIPs          []string `json:"allowed_ips"`
	PersistentKeepalive uint16   `json:"persistent_keepalive,omitempty"`
}

type NATMapping struct {
	Type              string `json:"type,omitempty"`
	ShadowPrefix      string `json:"shadow_prefix,omitempty"`
	RealPrefix        string `json:"real_prefix,omitempty"`
	SourcePrefix      string `json:"source_prefix,omitempty"`
	DestinationPrefix string `json:"destination_prefix,omitempty"`
	EgressInterface   string `json:"egress_interface,omitempty"`
	ToAddress         string `json:"to_address,omitempty"`
}

const (
	NATTypeDNAT = "dnat"
	NATTypeSNAT = "snat"
)

type ACLRule struct {
	ID           string `json:"id"`
	Priority     uint32 `json:"priority"`
	SourcePrefix string `json:"source_prefix"`
	Destination  string `json:"destination_prefix"`
	Protocol     string `json:"protocol"`
	PortFrom     uint16 `json:"port_from,omitempty"`
	PortTo       uint16 `json:"port_to,omitempty"`
	Action       string `json:"action"`
}

// Decode reads one strict JSON config document and validates its contents.
func Decode(r io.Reader) (Config, error) {
	return decode(r, false)
}

func decode(r io.Reader, allowPrivateKeyFile bool) (Config, error) {
	data, err := io.ReadAll(io.LimitReader(r, maxConfigBytes+1))
	if err != nil {
		return Config{}, fmt.Errorf("read gateway config: %w", err)
	}
	if len(data) > maxConfigBytes {
		return Config{}, fmt.Errorf("gateway config exceeds %d bytes", maxConfigBytes)
	}
	if err := rejectDuplicateFields(data); err != nil {
		return Config{}, fmt.Errorf("decode gateway config: %w", err)
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var cfg Config
	if err := decoder.Decode(&cfg); err != nil {
		return Config{}, fmt.Errorf("decode gateway config: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return Config{}, errors.New("gateway config contains multiple JSON values")
		}
		return Config{}, fmt.Errorf("decode trailing gateway config data: %w", err)
	}
	if cfg.Tunnel.PrivateKeyFile != "" {
		if !allowPrivateKeyFile {
			return Config{}, errors.New("tunnel.private_key_file is only supported when loading a private config file")
		}
		if cfg.Tunnel.PrivateKey != "" {
			return Config{}, errors.New("tunnel.private_key and tunnel.private_key_file cannot both be set")
		}
		if !validPrivateKeyFilename(cfg.Tunnel.PrivateKeyFile) {
			return Config{}, errors.New("tunnel.private_key_file must be a filename in the config directory")
		}
		validationCopy := cfg
		validationCopy.Tunnel.PrivateKey = "AQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQE="
		validationCopy.Tunnel.PrivateKeyFile = ""
		if err := validationCopy.Validate(); err != nil {
			return Config{}, err
		}
		return cfg.Canonical(), nil
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg.Canonical(), nil
}

func rejectDuplicateFields(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := scanJSONValue(decoder); err != nil {
		return err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple JSON values")
		}
		return err
	}
	return nil
}

func scanJSONValue(decoder *json.Decoder) error {
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
				return errors.New("object field name is not a string")
			}
			if _, exists := seen[key]; exists {
				return fmt.Errorf("duplicate object field %q", key)
			}
			seen[key] = struct{}{}
			if err := scanJSONValue(decoder); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil {
			return err
		}
		if closing != json.Delim('}') {
			return errors.New("malformed JSON object")
		}
	case '[':
		for decoder.More() {
			if err := scanJSONValue(decoder); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil {
			return err
		}
		if closing != json.Delim(']') {
			return errors.New("malformed JSON array")
		}
	default:
		return errors.New("unexpected JSON delimiter")
	}
	return nil
}

// Canonical returns an independent copy with network fields normalized for
// stable reconciliation and idempotency checks.
func (c Config) Canonical() Config {
	c.Tunnel.MTU = c.Tunnel.EffectiveMTU()
	c.Tunnel.Peers = append([]Peer(nil), c.Tunnel.Peers...)
	for i := range c.Tunnel.Peers {
		c.Tunnel.Peers[i].AllowedIPs = canonicalPrefixes(c.Tunnel.Peers[i].AllowedIPs)
		sort.Strings(c.Tunnel.Peers[i].AllowedIPs)
	}
	sort.Slice(c.Tunnel.Peers, func(i, j int) bool { return c.Tunnel.Peers[i].ID < c.Tunnel.Peers[j].ID })
	c.Routes = canonicalPrefixes(c.Routes)
	sort.Strings(c.Routes)
	c.NAT = append([]NATMapping(nil), c.NAT...)
	for i := range c.NAT {
		c.NAT[i].Type = c.NAT[i].normalizedType()
		switch c.NAT[i].Type {
		case NATTypeDNAT:
			c.NAT[i].ShadowPrefix = canonicalPrefix(c.NAT[i].ShadowPrefix)
			c.NAT[i].RealPrefix = canonicalPrefix(c.NAT[i].RealPrefix)
		case NATTypeSNAT:
			c.NAT[i].SourcePrefix = canonicalPrefix(c.NAT[i].SourcePrefix)
			c.NAT[i].DestinationPrefix = canonicalPrefix(c.NAT[i].DestinationPrefix)
		}
	}
	sort.Slice(c.NAT, func(i, j int) bool {
		if c.NAT[i].Type != c.NAT[j].Type {
			return c.NAT[i].Type < c.NAT[j].Type
		}
		if c.NAT[i].Type == NATTypeDNAT {
			if c.NAT[i].ShadowPrefix == c.NAT[j].ShadowPrefix {
				return c.NAT[i].RealPrefix < c.NAT[j].RealPrefix
			}
			return c.NAT[i].ShadowPrefix < c.NAT[j].ShadowPrefix
		}
		if c.NAT[i].SourcePrefix != c.NAT[j].SourcePrefix {
			return c.NAT[i].SourcePrefix < c.NAT[j].SourcePrefix
		}
		if c.NAT[i].DestinationPrefix != c.NAT[j].DestinationPrefix {
			return c.NAT[i].DestinationPrefix < c.NAT[j].DestinationPrefix
		}
		if c.NAT[i].EgressInterface != c.NAT[j].EgressInterface {
			return c.NAT[i].EgressInterface < c.NAT[j].EgressInterface
		}
		return c.NAT[i].ToAddress < c.NAT[j].ToAddress
	})
	c.Rules = append([]ACLRule(nil), c.Rules...)
	for i := range c.Rules {
		c.Rules[i].SourcePrefix = canonicalPrefix(c.Rules[i].SourcePrefix)
		c.Rules[i].Destination = canonicalPrefix(c.Rules[i].Destination)
		c.Rules[i].Protocol = strings.ToLower(c.Rules[i].Protocol)
		c.Rules[i].Action = strings.ToLower(c.Rules[i].Action)
	}
	sort.Slice(c.Rules, func(i, j int) bool { return c.Rules[i].Priority < c.Rules[j].Priority })
	return c
}

// LoadFile reads a private config file and any referenced key material only
// when the files are regular and grant no group or other access.
func LoadFile(path string) (Config, error) {
	cfg, _, err := LoadFileWithDigest(path)
	return cfg, err
}

// LoadFileWithDigest reads and validates a private config file and returns a
// fingerprint of its contents and any referenced private key. The digest is
// returned for parse errors too, so a watcher can suppress repeated failures.
func LoadFileWithDigest(path string) (Config, [sha256.Size]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return Config{}, [sha256.Size]byte{}, fmt.Errorf("stat gateway config: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return Config{}, [sha256.Size]byte{}, errors.New("gateway config must be a regular non-symlink file")
	}
	if info.Mode().Perm()&0o077 != 0 {
		return Config{}, [sha256.Size]byte{}, unsafePermissionsError("gateway config", path, info.Mode(), 0o600)
	}

	file, err := os.Open(path)
	if err != nil {
		return Config{}, [sha256.Size]byte{}, fmt.Errorf("open gateway config: %w", err)
	}
	defer file.Close()
	openedInfo, err := file.Stat()
	if err != nil {
		return Config{}, [sha256.Size]byte{}, fmt.Errorf("inspect gateway config: %w", err)
	}
	if !os.SameFile(info, openedInfo) || !openedInfo.Mode().IsRegular() {
		return Config{}, [sha256.Size]byte{}, errors.New("gateway config changed while opening")
	}
	if openedInfo.Mode().Perm()&0o077 != 0 {
		return Config{}, [sha256.Size]byte{}, unsafePermissionsError("gateway config permissions changed while opening", path, openedInfo.Mode(), 0o600)
	}
	data, err := io.ReadAll(io.LimitReader(file, maxConfigBytes+1))
	if err != nil {
		return Config{}, [sha256.Size]byte{}, fmt.Errorf("read gateway config: %w", err)
	}
	digest := sha256.Sum256(data)
	if len(data) > maxConfigBytes {
		return Config{}, digest, fmt.Errorf("gateway config exceeds %d bytes", maxConfigBytes)
	}
	cfg, err := decode(bytes.NewReader(data), true)
	if err != nil {
		return Config{}, digest, err
	}
	if cfg.Tunnel.PrivateKeyFile != "" {
		keyPath := filepath.Join(filepath.Dir(path), cfg.Tunnel.PrivateKeyFile)
		key, keyData, err := loadPrivateKeyFile(keyPath)
		if err != nil {
			return Config{}, digest, err
		}
		cfg.Tunnel.PrivateKey = key
		cfg.Tunnel.PrivateKeyFile = ""
		if err := cfg.Validate(); err != nil {
			return Config{}, digest, err
		}
		hash := sha256.New()
		_, _ = hash.Write(data)
		_, _ = hash.Write([]byte{0})
		_, _ = hash.Write(keyData)
		copy(digest[:], hash.Sum(nil))
	}
	return cfg.Canonical(), digest, nil
}

func validPrivateKeyFilename(name string) bool {
	return identifierPattern.MatchString(name)
}

func unsafePermissionsError(label, path string, mode, required os.FileMode) error {
	return fmt.Errorf("%s %q has unsafe permissions (%04o); run `chmod %03o %s`. ", label, path, mode.Perm(), required.Perm(), shellQuote(path))
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func loadPrivateKeyFile(path string) (string, []byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return "", nil, fmt.Errorf("stat WireGuard private key file: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return "", nil, errors.New("WireGuard private key file must be a regular non-symlink file")
	}
	if info.Mode().Perm()&0o077 != 0 {
		return "", nil, unsafePermissionsError("WireGuard private key file", path, info.Mode(), 0o600)
	}
	file, err := os.Open(path)
	if err != nil {
		return "", nil, fmt.Errorf("open WireGuard private key file: %w", err)
	}
	defer file.Close()
	openedInfo, err := file.Stat()
	if err != nil {
		return "", nil, fmt.Errorf("inspect WireGuard private key file: %w", err)
	}
	if !os.SameFile(info, openedInfo) || !openedInfo.Mode().IsRegular() {
		return "", nil, errors.New("WireGuard private key file changed while opening")
	}
	if openedInfo.Mode().Perm()&0o077 != 0 {
		return "", nil, unsafePermissionsError("WireGuard private key file permissions changed while opening", path, openedInfo.Mode(), 0o600)
	}
	data, err := io.ReadAll(io.LimitReader(file, 1025))
	if err != nil {
		return "", nil, fmt.Errorf("read WireGuard private key file: %w", err)
	}
	if len(data) > 1024 {
		return "", nil, errors.New("WireGuard private key file exceeds 1024 bytes")
	}
	key := strings.TrimSpace(string(data))
	if err := validateWireGuardKey(key); err != nil {
		return "", nil, fmt.Errorf("WireGuard private key file is invalid: %w", err)
	}
	return key, data, nil
}

func (c Config) Validate() error {
	if c.SchemaVersion != 1 {
		return fmt.Errorf("unsupported schema_version %d", c.SchemaVersion)
	}
	if !identifierPattern.MatchString(c.GatewayID) {
		return errors.New("gateway_id must be 1 to 128 safe ASCII characters")
	}
	if c.Generation == 0 {
		return errors.New("generation must be greater than zero")
	}
	if !interfacePattern.MatchString(c.Tunnel.Interface) || c.Tunnel.Interface == "." || c.Tunnel.Interface == ".." {
		return errors.New("tunnel.interface must be a valid Linux interface name")
	}
	if c.Tunnel.MTU != 0 && (c.Tunnel.MTU < 1280 || c.Tunnel.MTU > 9000) {
		return errors.New("tunnel.mtu must be between 1280 and 9000")
	}
	address, err := netip.ParsePrefix(c.Tunnel.Address)
	if err != nil || !address.IsValid() || address.Bits() == 0 || address.Addr().IsUnspecified() || address.Addr().IsMulticast() || address.Addr().Is4In6() {
		return errors.New("tunnel.address must be a valid unicast IP prefix")
	}
	if c.Tunnel.ListenPort == 0 {
		return errors.New("tunnel.listen_port must be between 1 and 65535")
	}
	if c.Tunnel.PrivateKeyFile != "" {
		return errors.New("tunnel.private_key_file must be resolved by LoadFile")
	}
	if err := validateWireGuardKey(c.Tunnel.PrivateKey); err != nil {
		return fmt.Errorf("tunnel.private_key is invalid: %w", err)
	}

	peerIDs := make(map[string]struct{}, len(c.Tunnel.Peers))
	peerKeys := make(map[string]struct{}, len(c.Tunnel.Peers))
	allowedIPOwners := make(map[netip.Prefix]string)
	var peerAllowedPrefixes []netip.Prefix
	for i, peer := range c.Tunnel.Peers {
		if !identifierPattern.MatchString(peer.ID) {
			return fmt.Errorf("tunnel.peers[%d].id is invalid", i)
		}
		if _, exists := peerIDs[peer.ID]; exists {
			return fmt.Errorf("tunnel.peers[%d].id is duplicated", i)
		}
		peerIDs[peer.ID] = struct{}{}
		if err := validateWireGuardKey(peer.PublicKey); err != nil {
			return fmt.Errorf("tunnel.peers[%d].public_key is invalid", i)
		}
		if _, exists := peerKeys[peer.PublicKey]; exists {
			return fmt.Errorf("tunnel.peers[%d].public_key is duplicated", i)
		}
		peerKeys[peer.PublicKey] = struct{}{}
		if peer.Endpoint != "" {
			if err := validateEndpoint(peer.Endpoint); err != nil {
				return fmt.Errorf("tunnel.peers[%d].endpoint is invalid: %w", i, err)
			}
		}
		if len(peer.AllowedIPs) == 0 {
			return fmt.Errorf("tunnel.peers[%d].allowed_ips must not be empty", i)
		}
		if err := validatePrefixes(peer.AllowedIPs, c.AllowDefaultRoute); err != nil {
			return fmt.Errorf("tunnel.peers[%d].allowed_ips: %w", i, err)
		}
		for _, rawPrefix := range peer.AllowedIPs {
			prefix, _ := netip.ParsePrefix(rawPrefix)
			prefix = prefix.Masked()
			if owner, exists := allowedIPOwners[prefix]; exists {
				return fmt.Errorf("tunnel.peers[%d].allowed_ips duplicates %s on peer %q", i, prefix, owner)
			}
			allowedIPOwners[prefix] = peer.ID
			peerAllowedPrefixes = append(peerAllowedPrefixes, prefix)
		}
	}

	if err := validatePrefixes(c.Routes, c.AllowDefaultRoute); err != nil {
		return fmt.Errorf("routes: %w", err)
	}
	for i, rawRoute := range c.Routes {
		route, _ := netip.ParsePrefix(rawRoute)
		route = route.Masked()
		if !routeCoveredByPeer(route, peerAllowedPrefixes) {
			return fmt.Errorf("routes[%d] %s is not covered by any tunnel.peers[].allowed_ips prefix", i, route)
		}
	}
	if err := validateNAT(c.NAT, c.Tunnel.Interface); err != nil {
		return err
	}
	if err := validateRules(c.Rules); err != nil {
		return err
	}
	return nil
}

func routeCoveredByPeer(route netip.Prefix, peerPrefixes []netip.Prefix) bool {
	for _, peerPrefix := range peerPrefixes {
		if peerPrefix.Addr().Is4() == route.Addr().Is4() && peerPrefix.Bits() <= route.Bits() && peerPrefix.Contains(route.Addr()) {
			return true
		}
	}
	return false
}

func validateWireGuardKey(value string) error {
	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil || len(decoded) != 32 {
		return errors.New("must be a base64-encoded 32-byte key")
	}
	if base64.StdEncoding.EncodeToString(decoded) != value {
		return errors.New("must use canonical base64 encoding")
	}
	allZero := true
	for _, b := range decoded {
		allZero = allZero && b == 0
	}
	if allZero {
		return errors.New("must not be zero")
	}
	return nil
}

func validateEndpoint(endpoint string) error {
	host, portText, err := net.SplitHostPort(endpoint)
	if err != nil {
		return errors.New("must use host:port format")
	}
	if host == "" || strings.ContainsAny(host, " \t\r\n/\\") {
		return errors.New("host is empty or malformed")
	}
	port, err := strconv.ParseUint(portText, 10, 16)
	if err != nil || port == 0 {
		return errors.New("port must be between 1 and 65535")
	}
	return nil
}

func validatePrefixes(values []string, allowDefault bool) error {
	seen := make(map[netip.Prefix]struct{}, len(values))
	for i, value := range values {
		prefix, err := netip.ParsePrefix(value)
		if err != nil || !prefix.IsValid() || prefix.Addr().Is4In6() || prefix.Addr().IsMulticast() || (prefix.Addr().IsUnspecified() && prefix.Bits() != 0) {
			return fmt.Errorf("prefix %d is invalid", i)
		}
		prefix = prefix.Masked()
		if prefix.Bits() == 0 && !allowDefault {
			return fmt.Errorf("prefix %d is a default route but allow_default_route is false", i)
		}
		if _, exists := seen[prefix]; exists {
			return fmt.Errorf("prefix %d is duplicated after masking", i)
		}
		seen[prefix] = struct{}{}
	}
	return nil
}

func (m NATMapping) normalizedType() string {
	if m.Type == "" {
		return NATTypeDNAT
	}
	return strings.ToLower(m.Type)
}

// Kind returns the effective NAT type. An omitted type preserves the legacy
// prefix DNAT configuration format.
func (m NATMapping) Kind() string {
	return m.normalizedType()
}

func validateNAT(mappings []NATMapping, tunnelInterface string) error {
	shadowPrefixes := make([]netip.Prefix, 0, len(mappings))
	snatRules := make([]NATMapping, 0, len(mappings))
	for i, mapping := range mappings {
		switch mapping.normalizedType() {
		case NATTypeDNAT:
			if mapping.SourcePrefix != "" || mapping.DestinationPrefix != "" || mapping.EgressInterface != "" || mapping.ToAddress != "" {
				return fmt.Errorf("nat[%d] DNAT mapping contains SNAT fields", i)
			}
			shadow, err := netip.ParsePrefix(mapping.ShadowPrefix)
			if err != nil || !shadow.IsValid() || shadow.Addr().Is4In6() {
				return fmt.Errorf("nat[%d].shadow_prefix is invalid", i)
			}
			real, err := netip.ParsePrefix(mapping.RealPrefix)
			if err != nil || !real.IsValid() || real.Addr().Is4In6() {
				return fmt.Errorf("nat[%d].real_prefix is invalid", i)
			}
			shadow = shadow.Masked()
			real = real.Masked()
			if shadow.Addr().Is4() != real.Addr().Is4() || shadow.Bits() != real.Bits() {
				return fmt.Errorf("nat[%d] prefixes must have matching address families and lengths", i)
			}
			for _, existing := range shadowPrefixes {
				if existing.Overlaps(shadow) {
					return fmt.Errorf("nat[%d].shadow_prefix overlaps another mapping", i)
				}
			}
			shadowPrefixes = append(shadowPrefixes, shadow)
		case NATTypeSNAT:
			if mapping.ShadowPrefix != "" || mapping.RealPrefix != "" {
				return fmt.Errorf("nat[%d] SNAT mapping contains DNAT fields", i)
			}
			source, err := netip.ParsePrefix(mapping.SourcePrefix)
			if err != nil || !source.IsValid() || source.Addr().Is4In6() || source.Bits() == 0 {
				return fmt.Errorf("nat[%d].source_prefix is invalid or too broad", i)
			}
			destination, err := netip.ParsePrefix(mapping.DestinationPrefix)
			if err != nil || !destination.IsValid() || destination.Addr().Is4In6() || destination.Bits() == 0 {
				return fmt.Errorf("nat[%d].destination_prefix is invalid or too broad", i)
			}
			source = source.Masked()
			destination = destination.Masked()
			if source.Addr().Is4() != destination.Addr().Is4() {
				return fmt.Errorf("nat[%d] source and destination prefix address families differ", i)
			}
			if !interfacePattern.MatchString(mapping.EgressInterface) || mapping.EgressInterface == "." || mapping.EgressInterface == ".." {
				return fmt.Errorf("nat[%d].egress_interface is invalid", i)
			}
			if mapping.EgressInterface == tunnelInterface {
				return fmt.Errorf("nat[%d].egress_interface must differ from the tunnel interface", i)
			}
			address, err := netip.ParseAddr(mapping.ToAddress)
			if err != nil || !address.IsValid() || address.Is4In6() || address.Zone() != "" || !address.IsGlobalUnicast() {
				return fmt.Errorf("nat[%d].to_address must be a unicast IP address", i)
			}
			if address.Is4() != source.Addr().Is4() {
				return fmt.Errorf("nat[%d].to_address address family differs from prefixes", i)
			}
			for previous, existing := range snatRules {
				existingSource, _ := netip.ParsePrefix(existing.SourcePrefix)
				existingDestination, _ := netip.ParsePrefix(existing.DestinationPrefix)
				existingSource = existingSource.Masked()
				existingDestination = existingDestination.Masked()
				if existing.EgressInterface == mapping.EgressInterface && existingSource.Overlaps(source) && existingDestination.Overlaps(destination) {
					return fmt.Errorf("nat[%d] overlaps SNAT mapping nat[%d]", i, previous)
				}
			}
			snatRules = append(snatRules, mapping)
		default:
			return fmt.Errorf("nat[%d].type must be %q or %q", i, NATTypeDNAT, NATTypeSNAT)
		}
	}
	return nil
}

func validateRules(rules []ACLRule) error {
	ids := make(map[string]struct{}, len(rules))
	for i, rule := range rules {
		if !identifierPattern.MatchString(rule.ID) {
			return fmt.Errorf("rules[%d].id is invalid", i)
		}
		if _, exists := ids[rule.ID]; exists {
			return fmt.Errorf("rules[%d].id is duplicated", i)
		}
		ids[rule.ID] = struct{}{}
		sourcePrefix, err := netip.ParsePrefix(rule.SourcePrefix)
		if err != nil || sourcePrefix.Addr().Is4In6() {
			return fmt.Errorf("rules[%d].source_prefix is invalid", i)
		}
		destinationPrefix, err := netip.ParsePrefix(rule.Destination)
		if err != nil || destinationPrefix.Addr().Is4In6() {
			return fmt.Errorf("rules[%d].destination_prefix is invalid", i)
		}
		if sourcePrefix.Addr().Is4() != destinationPrefix.Addr().Is4() {
			return fmt.Errorf("rules[%d] source and destination address families differ", i)
		}
		protocol := strings.ToLower(rule.Protocol)
		switch protocol {
		case "any":
			if rule.PortFrom != 0 || rule.PortTo != 0 {
				return fmt.Errorf("rules[%d] protocol %q cannot have ports", i, protocol)
			}
		case "icmp":
			if sourcePrefix.Addr().Is6() || rule.PortFrom != 0 || rule.PortTo != 0 {
				return fmt.Errorf("rules[%d] icmp requires IPv4 prefixes and cannot have ports", i)
			}
		case "icmpv6":
			if sourcePrefix.Addr().Is4() || rule.PortFrom != 0 || rule.PortTo != 0 {
				return fmt.Errorf("rules[%d] icmpv6 requires IPv6 prefixes and cannot have ports", i)
			}
		case "tcp", "udp":
			if (rule.PortFrom == 0) != (rule.PortTo == 0) || rule.PortFrom > rule.PortTo {
				return fmt.Errorf("rules[%d] port range is invalid", i)
			}
		default:
			return fmt.Errorf("rules[%d].protocol is unsupported", i)
		}
		action := strings.ToLower(rule.Action)
		if action != "allow" && action != "deny" {
			return fmt.Errorf("rules[%d].action must be allow or deny", i)
		}
		for previous := 0; previous < i; previous++ {
			if rules[previous].Priority == rule.Priority {
				return fmt.Errorf("rules[%d].priority is duplicated", i)
			}
		}
	}
	return nil
}

func canonicalPrefixes(values []string) []string {
	result := append([]string(nil), values...)
	for i := range result {
		result[i] = canonicalPrefix(result[i])
	}
	return result
}

func canonicalPrefix(value string) string {
	prefix, err := netip.ParsePrefix(value)
	if err != nil {
		return value
	}
	return prefix.Masked().String()
}
