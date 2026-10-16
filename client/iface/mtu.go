// SPDX-License-Identifier: BSD-3-Clause
//
// ValidateMTU and its bounds are adapted from an upstream WireGuard interface
// MTU validation. Platform-specific interface creation remains VeloTrust-owned.
package iface

import "fmt"

const (
	DefaultMTU = 1280
	MinMTU     = 576
	MaxMTU     = 8192
)

// ValidateMTU validates that MTU is within the generic WireGuard tunnel range.
func ValidateMTU(mtu uint16) error {
	if mtu < MinMTU {
		return fmt.Errorf("MTU %d below minimum (%d bytes)", mtu, MinMTU)
	}
	if mtu > MaxMTU {
		return fmt.Errorf("MTU %d exceeds maximum supported size (%d bytes)", mtu, MaxMTU)
	}
	return nil
}
