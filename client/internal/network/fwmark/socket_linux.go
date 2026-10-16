//go:build linux && !android

// SPDX-License-Identifier: BSD-3-Clause

package fwmark

import (
	"fmt"
	"syscall"
)

// SetSocketMark marks a socket so host routing rules can keep its traffic outside the tunnel.
func SetSocketMark(conn syscall.Conn) error {
	if conn == nil {
		return fmt.Errorf("cannot mark a nil socket")
	}
	raw, err := conn.SyscallConn()
	if err != nil {
		return fmt.Errorf("get raw socket: %w", err)
	}
	var setErr error
	if err := raw.Control(func(fd uintptr) {
		setErr = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_MARK, int(ControlPlane))
	}); err != nil {
		return fmt.Errorf("control socket: %w", err)
	}
	if setErr != nil {
		return fmt.Errorf("set socket mark: %w", setErr)
	}
	return nil
}
