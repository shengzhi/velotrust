//go:build !linux || android

// SPDX-License-Identifier: BSD-3-Clause

package fwmark

import (
	"errors"
	"syscall"
)

var ErrUnsupported = errors.New("socket fwmarks are unavailable on this platform")

func SetSocketMark(syscall.Conn) error { return ErrUnsupported }
