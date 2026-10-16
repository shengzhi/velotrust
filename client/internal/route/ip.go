// SPDX-License-Identifier: BSD-3-Clause

package route

import (
	"fmt"
	"net"
	"net/netip"
)

// PrefixFromIP converts an IP address to its host prefix, normalizing mapped
// IPv4 addresses to IPv4 form.
func PrefixFromIP(ip net.IP) (netip.Prefix, error) {
	addr, ok := netip.AddrFromSlice(ip)
	if !ok {
		return netip.Prefix{}, fmt.Errorf("parse IP address: %s", ip)
	}
	addr = addr.Unmap()
	return netip.PrefixFrom(addr, addr.BitLen()), nil
}
