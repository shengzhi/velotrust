package main

import (
	"context"
	"testing"

	"velotrust/shared/configpayload"
	hubv1 "velotrust/shared/gen/go/hub/v1"
)

func TestApplyEmptyGatewaySetStopsTunnelAndAcknowledgesSnapshot(t *testing.T) {
	payload := []byte(`{"schema_version":1,"tunnel":{"address":"10.100.0.2/32","mtu":1380,"peers":[]}}`)
	hash, err := configpayload.Hash(payload)
	if err != nil {
		t.Fatal(err)
	}

	manager := newHubTunnelManager("test", t.TempDir(), "client.key", nil)
	done := make(chan error, 1)
	done <- context.Canceled
	manager.active = &managedTunnelRun{cancel: func() {}, done: done}

	err = manager.Apply(context.Background(), &hubv1.ConfigSnapshot{ConfigHash: hash, Payload: payload})
	if err != nil {
		t.Fatalf("apply empty Gateway set: %v", err)
	}
	status := manager.Status()
	if status.AppliedConfigHash != hash {
		t.Fatalf("applied hash = %q, want %q", status.AppliedConfigHash, hash)
	}
	if status.WireGuardStatus != "down" || status.LastErrorCode != "no_authorized_gateway" {
		t.Fatalf("tunnel status = %q, error = %q; want down/no_authorized_gateway", status.WireGuardStatus, status.LastErrorCode)
	}
	if manager.active != nil {
		t.Fatal("Client tunnel remained active after receiving an empty Gateway set")
	}
}
