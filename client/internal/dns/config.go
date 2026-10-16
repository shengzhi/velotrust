// SPDX-License-Identifier: BSD-3-Clause
//
// This file contains independent DNS helpers adapted from a BSD-3-Clause
// upstream DNS package. VeloTrust owns the configuration types below.
package dns

import (
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/net/idna"
)

const (
	// TypeA is the DNS record type for an IPv4 address.
	TypeA = 1
	// TypeCNAME is the DNS record type for a canonical name.
	TypeCNAME = 5
	// TypeAAAA is the DNS record type for an IPv6 address.
	TypeAAAA         = 28
	invalidHostLabel = "[^a-zA-Z0-9-]+"
)

// Config is the agent-side DNS configuration. It is a VeloTrust model and is
// deliberately independent of management database entities and wire formats.
type Config struct {
	Enabled        bool
	ResolverGroups []ResolverGroup
	Zones          []CustomZone
}

// ResolverGroup groups DNS resolvers and the domains routed to them.
type ResolverGroup struct {
	Resolvers            []netip.AddrPort
	Domains              []string
	Primary              bool
	SearchDomainsEnabled bool
}

// ParseResolverURL parses a UDP resolver endpoint in udp://<ip>:<port> form.
func ParseResolverURL(value string) (netip.AddrPort, error) {
	parsed, err := url.Parse(value)
	if err != nil {
		return netip.AddrPort{}, fmt.Errorf("parse resolver URL: %w", err)
	}
	if !strings.EqualFold(parsed.Scheme, "udp") || parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return netip.AddrPort{}, fmt.Errorf("invalid resolver URL %q", value)
	}
	address, err := netip.ParseAddr(parsed.Hostname())
	if err != nil {
		return netip.AddrPort{}, fmt.Errorf("invalid resolver IP %q: %w", parsed.Hostname(), err)
	}
	port, err := strconv.ParseUint(parsed.Port(), 10, 16)
	if err != nil || port == 0 {
		return netip.AddrPort{}, fmt.Errorf("invalid resolver port %q", parsed.Port())
	}
	return netip.AddrPortFrom(address, uint16(port)), nil
}

// CustomZone represents a custom zone to be resolved by the DNS server.
type CustomZone struct {
	Domain               string
	Records              []SimpleRecord
	SearchDomainDisabled bool
	NonAuthoritative     bool
}

// SimpleRecord provides a simple DNS record specification for CNAME, A and AAAA records.
type SimpleRecord struct {
	Name  string
	Type  int
	Class string
	TTL   int
	RData string
}

var invalidHostMatcher = regexp.MustCompile(invalidHostLabel)

// GetParsedDomainLabel returns the first domain label converted to ASCII and
// lowercase, with invalid host characters replaced by hyphens.
//
// The IDNA conversion and sanitization are adapted from an upstream DNS helper.
func GetParsedDomainLabel(name string) (string, error) {
	trimmed := strings.TrimSuffix(name, ".")
	if trimmed == "" {
		return "", fmt.Errorf("got empty label list for name %q", name)
	}
	labels := strings.Split(trimmed, ".")
	if len(labels) == 0 || labels[0] == "" {
		return "", fmt.Errorf("got empty label list for name %q", name)
	}
	label, err := idna.Punycode.ToASCII(labels[0])
	if err != nil {
		return "", fmt.Errorf("unable to convert host label to ASCII: %w", err)
	}
	validHost := strings.ToLower(invalidHostMatcher.ReplaceAllString(label, "-"))
	if len(validHost) > 58 {
		validHost = validHost[:59]
	}
	return validHost, nil
}

// NormalizeZone removes the wildcard prefix from a DNS zone name.
func NormalizeZone(domain string) string {
	return strings.TrimPrefix(domain, "*.")
}

// String formats the record as a DNS zone-file record.
func (record SimpleRecord) String() string {
	return fmt.Sprintf("%s %d %s %s %s", fqdn(record.Name), record.TTL,
		record.Class, recordTypeName(record.Type), record.RData)
}

// Len returns the RData length implied by the record type.
func (record SimpleRecord) Len() uint16 {
	if record.RData == "" {
		if record.Type == TypeCNAME {
			return 1
		}
		return 0
	}
	switch record.Type {
	case TypeA:
		return net.IPv4len
	case TypeCNAME:
		if record.RData == "." {
			return 1
		}
		return uint16(len(record.RData) + 1)
	case TypeAAAA:
		return net.IPv6len
	default:
		return 0
	}
}

func fqdn(name string) string {
	if strings.HasSuffix(name, ".") {
		return name
	}
	return name + "."
}

func recordTypeName(recordType int) string {
	switch recordType {
	case TypeA:
		return "A"
	case TypeCNAME:
		return "CNAME"
	case TypeAAAA:
		return "AAAA"
	default:
		return fmt.Sprintf("TYPE%d", recordType)
	}
}
