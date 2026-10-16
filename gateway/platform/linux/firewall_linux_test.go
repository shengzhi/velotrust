//go:build linux

package linux

import (
	"errors"
	"net/netip"
	"os"
	"syscall"
	"testing"

	"github.com/google/nftables"
	"github.com/google/nftables/expr"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
	"velotrust/gateway/config"
)

func TestFirewallRuleExpressionsCompileScopedTCPAllow(t *testing.T) {
	rule := config.ACLRule{
		ID: "https", SourcePrefix: "10.8.0.0/16", Destination: "192.0.2.0/24",
		Protocol: "tcp", PortFrom: 443, PortTo: 443, Action: "allow",
	}
	expressions, err := firewallRuleExpressions("vtrust0", rule)
	if err != nil {
		t.Fatal(err)
	}
	if len(expressions) < 12 {
		t.Fatalf("expected interface, prefix, protocol, port, and verdict expressions; got %d", len(expressions))
	}
	verdict, ok := expressions[len(expressions)-1].(*expr.Verdict)
	if !ok || verdict.Kind != expr.VerdictAccept {
		t.Fatalf("last expression = %#v, want accept verdict", expressions[len(expressions)-1])
	}
	var source, destination, portLoads int
	for _, expression := range expressions {
		payload, ok := expression.(*expr.Payload)
		if !ok {
			continue
		}
		if payload.Base == expr.PayloadBaseNetworkHeader && payload.Offset == 12 {
			source++
		}
		if payload.Base == expr.PayloadBaseNetworkHeader && payload.Offset == 16 {
			destination++
		}
		if payload.Base == expr.PayloadBaseTransportHeader && payload.Offset == 2 && payload.Len == 2 {
			portLoads++
		}
	}
	if source != 1 || destination != 1 || portLoads != 1 {
		t.Fatalf("unexpected payload matches: source=%d destination=%d port=%d", source, destination, portLoads)
	}
	assertExpressionsMarshal(t, expressions)
}

func TestNATRuleMapsWholePrefixForIPv6(t *testing.T) {
	expressions, err := natRuleExpressions("vtrust0", config.NATMapping{
		ShadowPrefix: "fd00:1::/120", RealPrefix: "fd10:1::/120",
	})
	if err != nil {
		t.Fatal(err)
	}
	nat, ok := expressions[len(expressions)-1].(*expr.NAT)
	if !ok {
		t.Fatalf("last expression = %#v, want NAT", expressions[len(expressions)-1])
	}
	if !nat.Prefix || nat.Family != unix.NFPROTO_IPV6 {
		t.Fatalf("NAT expression = %+v, want IPv6 prefix translation", nat)
	}
	if nat.RegAddrMin != unix.NFT_REG_1 || nat.RegAddrMax != unix.NFT_REG_2 {
		t.Fatalf("NAT registers = %d..%d, want 1..2", nat.RegAddrMin, nat.RegAddrMax)
	}
	assertExpressionsMarshal(t, expressions)
}

func TestReplyRuleMatchesReverseTupleAndCurrentAuthorization(t *testing.T) {
	rule := config.ACLRule{
		ID: "https", SourcePrefix: "10.8.0.0/16", Destination: "192.0.2.0/24",
		Protocol: "tcp", PortFrom: 443, PortTo: 443, Action: "allow",
	}
	expressions, err := firewallReplyExpressions("vtrust0", rule)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := expressions[0].(*expr.Meta); !ok {
		t.Fatalf("reply interface expression = %#v", expressions[0])
	}
	if got := expressions[0].(*expr.Meta).Key; got != expr.MetaKeyOIFNAME {
		t.Fatalf("reply interface key = %v, want output interface", got)
	}
	if _, ok := expressions[2].(*expr.Ct); !ok {
		t.Fatalf("reply state expression = %#v, want conntrack", expressions[2])
	}
	var sourceOffset, destinationOffset, sourcePortOffset int
	for _, expression := range expressions {
		payload, ok := expression.(*expr.Payload)
		if !ok {
			continue
		}
		if payload.Base == expr.PayloadBaseNetworkHeader && payload.Offset == 12 {
			sourceOffset++
		}
		if payload.Base == expr.PayloadBaseNetworkHeader && payload.Offset == 16 {
			destinationOffset++
		}
		if payload.Base == expr.PayloadBaseTransportHeader && payload.Offset == 0 {
			sourcePortOffset++
		}
	}
	if sourceOffset != 1 || destinationOffset != 1 || sourcePortOffset != 1 {
		t.Fatalf("reply tuple did not reverse expected fields: src=%d dst=%d sport=%d", sourceOffset, destinationOffset, sourcePortOffset)
	}
	assertExpressionsMarshal(t, expressions)
}

func TestKernelFallbackClassificationIsNarrow(t *testing.T) {
	if !isKernelFeatureUnavailable(syscall.EOPNOTSUPP) {
		t.Fatal("operation-not-supported should permit userspace fallback")
	}
	if !isKernelFeatureUnavailable(os.ErrNotExist) {
		t.Fatal("missing kernel device family should permit userspace fallback")
	}
	if isKernelFeatureUnavailable(syscall.EPERM) {
		t.Fatal("permission failure must not trigger userspace fallback")
	}
	if isKernelFeatureUnavailable(errors.New("invalid WireGuard private key")) {
		t.Fatal("configuration errors must not trigger userspace fallback")
	}
}

func TestLinkNotFoundRecognizesNetlinkError(t *testing.T) {
	_, err := netlink.LinkByName("vtrust-test-link-not-found")
	if err == nil {
		t.Fatal("test interface unexpectedly exists")
	}
	var notFound netlink.LinkNotFoundError
	if !errors.As(err, &notFound) {
		t.Skipf("netlink did not return a link-not-found error: %v", err)
	}
	if !isLinkNotFound(err) {
		t.Fatalf("isLinkNotFound(%T) = false, want true", err)
	}
}

func TestPrefixLastAddress(t *testing.T) {
	if got := prefixLastAddress(mustPrefix(t, "192.0.2.64/26")).String(); got != "192.0.2.127" {
		t.Fatalf("IPv4 last address = %s", got)
	}
	if got := prefixLastAddress(mustPrefix(t, "2001:db8::/126")).String(); got != "2001:db8::3" {
		t.Fatalf("IPv6 last address = %s", got)
	}
}

func mustPrefix(t *testing.T, value string) netip.Prefix {
	t.Helper()
	prefix, err := netip.ParsePrefix(value)
	if err != nil {
		t.Fatal(err)
	}
	return prefix
}

func assertExpressionsMarshal(t *testing.T, expressions []expr.Any) {
	t.Helper()
	for index, expression := range expressions {
		if _, err := expr.Marshal(byte(nftables.TableFamilyINet), expression); err != nil {
			t.Fatalf("marshal expression %d (%T): %v", index, expression, err)
		}
	}
}
