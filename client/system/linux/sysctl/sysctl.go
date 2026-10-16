// SPDX-License-Identifier: BSD-3-Clause

package sysctl

import (
	"strings"

	"velotrust/client/iface/wgaddr"
)

// Interface identifies the tunnel interface excluded from per-interface rp_filter changes.
type Interface interface {
	Address() wgaddr.Address
	Name() string
}

// EscapeInterfaceName protects dots and percent signs before a sysctl key is converted to a path.
func EscapeInterfaceName(name string) string {
	safe := strings.ReplaceAll(name, "%", "%25")
	return strings.ReplaceAll(safe, ".", "%2E")
}
