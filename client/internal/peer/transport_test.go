package peer

import "testing"

func TestEvaluateConnectionStatus(t *testing.T) {
	tests := []struct {
		name string
		in   ConnectionStatusInput
		want ConnectionStatus
	}{
		{
			name: "direct and required relay are up",
			in: ConnectionStatusInput{
				PeerUsesRelay:        true,
				RelayConnected:       true,
				RemoteSupportsDirect: true,
				DirectWorkerReady:    true,
				DirectConnecting:     true,
			},
			want: ConnectionConnected,
		},
		{
			name: "relay up while direct is down",
			in: ConnectionStatusInput{
				PeerUsesRelay:        true,
				RelayConnected:       true,
				RemoteSupportsDirect: true,
				DirectWorkerReady:    true,
			},
			want: ConnectionPartiallyConnected,
		},
		{
			name: "direct up but required relay down",
			in: ConnectionStatusInput{
				PeerUsesRelay:        true,
				RemoteSupportsDirect: true,
				DirectWorkerReady:    true,
				DirectConnecting:     true,
			},
			want: ConnectionDisconnected,
		},
		{
			name: "forced relay with no relay path",
			in: ConnectionStatusInput{
				ForceRelay:       true,
				RelayConnected:   true,
				DirectConnecting: true,
			},
			want: ConnectionDisconnected,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := EvaluateConnectionStatus(test.in); got != test.want {
				t.Fatalf("EvaluateConnectionStatus() = %s, want %s", got, test.want)
			}
		})
	}
}

func TestSessionIDRoundTrip(t *testing.T) {
	want := []byte{1, 2, 3, 4, 5}
	id, err := SessionIDFromBytes(want)
	if err != nil {
		t.Fatal(err)
	}
	got, err := id.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("round trip = %v, want %v", got, want)
		}
	}
	if _, err := NewSessionID(); err != nil {
		t.Fatalf("NewSessionID() error = %v", err)
	}
}

func TestSessionIDRejectsMalformedValues(t *testing.T) {
	if _, err := SessionIDFromBytes([]byte{1}); err == nil {
		t.Fatal("SessionIDFromBytes accepted a short value")
	}
	for _, id := range []SessionID{"", "not-hex!!", "0102"} {
		if _, err := id.Bytes(); err == nil {
			t.Errorf("Bytes() accepted %q", id)
		}
	}
}
