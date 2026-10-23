package postgres

import (
	"strings"
	"testing"
)

func TestNodeAuditActorUsesTypedStableIdentity(t *testing.T) {
	tests := []struct {
		name     string
		nodeType string
		nodeCode string
		want     string
	}{
		{name: "gateway", nodeType: "gateway", nodeCode: "gw-hk-01", want: "gateway:gw-hk-01"},
		{name: "client", nodeType: "client_agent", nodeCode: "cl_abc123", want: "client:cl_abc123"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := nodeAuditActor(test.nodeType, test.nodeCode); got != test.want {
				t.Fatalf("nodeAuditActor() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestAuditActorsRespectColumnLimit(t *testing.T) {
	longCode := strings.Repeat("n", 64)
	for _, actor := range []string{
		nodeAuditActor("gateway", longCode),
		nodeAuditActor("client_agent", longCode),
		auditActor(strings.Repeat("h", 100)),
	} {
		if len(actor) == 0 || len(actor) > maxAuditActorLength {
			t.Fatalf("audit actor length = %d, want 1..%d (%q)", len(actor), maxAuditActorLength, actor)
		}
	}
	first := nodeAuditActor("gateway", longCode)
	second := nodeAuditActor("gateway", longCode)
	if first != second || !strings.HasPrefix(first, "gateway:") {
		t.Fatalf("long node actor is not stable and typed: %q, %q", first, second)
	}
}

func TestNodeAuditActorRejectsUnknownNodeType(t *testing.T) {
	if got := nodeAuditActor("relay", "relay-1"); got != "" {
		t.Fatalf("nodeAuditActor() = %q, want empty for unsupported node type", got)
	}
}
