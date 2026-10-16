//go:build !linux || android

// SPDX-License-Identifier: BSD-3-Clause

package sharedsock

import (
	"errors"
	"net/netip"
)

var ErrProbeUnsupported = errors.New("source probe is supported only on Linux")

type SourceProbe struct{}

func NewSourceProbe(int, uint32) (*SourceProbe, error) { return nil, ErrProbeUnsupported }
func (*SourceProbe) Resolve(netip.Addr) (netip.Addr, error) {
	return netip.Addr{}, ErrProbeUnsupported
}
func (*SourceProbe) Close() error { return nil }
