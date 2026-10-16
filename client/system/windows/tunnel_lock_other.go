//go:build !windows

// SPDX-License-Identifier: BSD-3-Clause

package windows

import "errors"

func AcquireTunnelLock() (func() error, error) {
	return nil, errors.New("Windows client tunnel locking is available only on Windows")
}
