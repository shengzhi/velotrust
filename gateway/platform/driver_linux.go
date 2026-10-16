//go:build linux

package platform

import (
	"fmt"

	"velotrust/gateway/internal/agent"
	linuxdriver "velotrust/gateway/platform/linux"
)

// NewDriver creates the Linux networking backend for the managed tunnel.
func NewDriver(interfaceName string) (agent.Driver, error) {
	if interfaceName == "" {
		return nil, fmt.Errorf("managed tunnel interface is required")
	}
	return linuxdriver.New(interfaceName), nil
}
