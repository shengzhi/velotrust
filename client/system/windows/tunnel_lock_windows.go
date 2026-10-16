//go:build windows

// SPDX-License-Identifier: BSD-3-Clause

package windows

import (
	"errors"
	"fmt"
	"runtime"
	"sync"

	"golang.org/x/sys/windows"
)

// AcquireTunnelLock prevents two foreground clients from reconfiguring the
// shared VeloTrust Wintun adapter at the same time. The returned function must
// be called by the same goroutine because Windows mutex ownership is thread-bound.
func AcquireTunnelLock() (func() error, error) {
	runtime.LockOSThread()
	name, err := windows.UTF16PtrFromString(`Global\VeloTrustClientTunnel`)
	if err != nil {
		runtime.UnlockOSThread()
		return nil, err
	}
	handle, err := windows.CreateMutex(nil, false, name)
	if err != nil {
		runtime.UnlockOSThread()
		return nil, fmt.Errorf("create client tunnel lock: %w", err)
	}
	state, err := windows.WaitForSingleObject(handle, 0)
	if err != nil || (state != windows.WAIT_OBJECT_0 && state != windows.WAIT_ABANDONED) {
		_ = windows.CloseHandle(handle)
		runtime.UnlockOSThread()
		if err != nil {
			return nil, fmt.Errorf("acquire client tunnel lock: %w", err)
		}
		return nil, errors.New("another VeloTrust client tunnel session is already running")
	}
	var once sync.Once
	var releaseErr error
	return func() error {
		once.Do(func() {
			releaseErr = errors.Join(windows.ReleaseMutex(handle), windows.CloseHandle(handle))
			runtime.UnlockOSThread()
		})
		return releaseErr
	}, nil
}
