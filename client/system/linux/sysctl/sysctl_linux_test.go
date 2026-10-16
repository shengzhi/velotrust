//go:build linux && !android

package sysctl

import "testing"

func TestEscapeInterfaceName(t *testing.T) {
	for _, test := range []struct {
		name string
		want string
	}{
		{name: "eth0.100", want: "eth0%2E100"},
		{name: "vlan%blue.2", want: "vlan%25blue%2E2"},
	} {
		if got := EscapeInterfaceName(test.name); got != test.want {
			t.Errorf("EscapeInterfaceName(%q) = %q, want %q", test.name, got, test.want)
		}
	}
}
