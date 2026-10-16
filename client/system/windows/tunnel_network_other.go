//go:build !windows

// SPDX-License-Identifier: BSD-3-Clause

package windows

import "errors"

func ConfigureTunnelNetwork(string, string, []string) (func() error, error) {
	return nil, errors.New("Windows client tunnel networking is available only on Windows")
}

func CleanupTunnelNetwork(string) error {
	return errors.New("Windows client tunnel cleanup is available only on Windows")
}
