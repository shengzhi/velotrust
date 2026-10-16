//go:build linux && !android

// SPDX-License-Identifier: BSD-3-Clause

package sysctl

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strconv"
	"strings"
)

const (
	rpFilterPath          = "net.ipv4.conf.all.rp_filter"
	rpFilterInterfacePath = "net.ipv4.conf.%s.rp_filter"
	srcValidMarkPath      = "net.ipv4.conf.all.src_valid_mark"
	dotEscape             = "%2E"
	percentEscape         = "%25"
)

// Setup enables source validation for marked traffic and loosens reverse-path filtering.
// The returned map records original values for Cleanup.
func Setup(tunnel Interface) (map[string]int, error) {
	original := make(map[string]int)
	var failures []error
	for key, value := range map[string]int{srcValidMarkPath: 1, rpFilterPath: 2} {
		old, err := Set(key, value, key == rpFilterPath)
		if err != nil {
			failures = append(failures, err)
			continue
		}
		original[key] = old
	}

	interfaces, err := net.Interfaces()
	if err != nil {
		failures = append(failures, fmt.Errorf("list interfaces: %w", err))
	} else {
		for _, intf := range interfaces {
			if intf.Name == "lo" || tunnel != nil && intf.Name == tunnel.Name() {
				continue
			}
			key := fmt.Sprintf(rpFilterInterfacePath, EscapeInterfaceName(intf.Name))
			old, err := Set(key, 2, true)
			if err != nil {
				failures = append(failures, err)
				continue
			}
			original[key] = old
		}
	}
	return original, errors.Join(failures...)
}

// Set updates a sysctl value. If onlyIfOne is true, it changes the value only when the current value is 1.
func Set(key string, desiredValue int, onlyIfOne bool) (int, error) {
	path := strings.ReplaceAll(key, ".", "/")
	path = strings.ReplaceAll(path, dotEscape, ".")
	path = strings.ReplaceAll(path, percentEscape, "%")
	path = "/proc/sys/" + path
	currentValue, err := os.ReadFile(path)
	if err != nil {
		return -1, fmt.Errorf("read sysctl %s: %w", key, err)
	}
	current, err := strconv.Atoi(strings.TrimSpace(string(currentValue)))
	if err != nil {
		return -1, fmt.Errorf("parse current sysctl value for %s: %w", key, err)
	}
	if current == desiredValue || onlyIfOne && current != 1 {
		return current, nil
	}
	if err := os.WriteFile(path, []byte(strconv.Itoa(desiredValue)), 0o644); err != nil {
		return current, fmt.Errorf("write sysctl %s: %w", key, err)
	}
	slog.Debug("updated sysctl", "key", key, "from", current, "to", desiredValue)
	return current, nil
}

// Cleanup restores the values recorded by Setup.
func Cleanup(original map[string]int) error {
	var failures []error
	for key, value := range original {
		if _, err := Set(key, value, false); err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}
