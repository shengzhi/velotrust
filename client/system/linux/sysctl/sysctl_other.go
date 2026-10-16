//go:build !linux || android

// SPDX-License-Identifier: BSD-3-Clause

package sysctl

import "errors"

var ErrUnsupported = errors.New("Linux sysctl configuration is unavailable on this platform")

func Setup(Interface) (map[string]int, error) { return nil, ErrUnsupported }

func Set(string, int, bool) (int, error) { return -1, ErrUnsupported }

func Cleanup(map[string]int) error { return ErrUnsupported }
