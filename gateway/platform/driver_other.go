//go:build !linux

package platform

import (
	"errors"

	"velotrust/gateway/internal/agent"
)

// NewDriver reports that the live gateway dataplane is currently Linux-only.
func NewDriver(string) (agent.Driver, error) {
	return nil, errors.New("live gateway dataplane is supported on Linux only")
}
