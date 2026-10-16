//go:build !windows

// SPDX-License-Identifier: BSD-3-Clause

package windows

import (
	"errors"
	"net/netip"
)

func DiscoverSystemDNS() ([]netip.AddrPort, error) {
	return nil, errors.New("system DNS discovery is available only on Windows")
}

func ApplyDNSCapture(string, string) (func() error, error) {
	return nil, errors.New("DNS capture is available only on Windows")
}

func CleanupDNSCapture(string) error {
	return errors.New("DNS capture cleanup is available only on Windows")
}

func DNSCaptureProbeName() (string, error) {
	return "", errors.New("DNS capture is available only on Windows")
}
