//go:build !windows

// SPDX-License-Identifier: BSD-3-Clause

package windows

import (
	"errors"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

func GenerateWireGuardKey(string) (string, error) {
	return "", errors.New("WireGuard key generation is available only on Windows")
}

func LoadWireGuardPrivateKey(string) (wgtypes.Key, error) {
	return wgtypes.Key{}, errors.New("Windows client private-key loading is available only on Windows")
}
