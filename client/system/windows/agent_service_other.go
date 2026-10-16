//go:build !windows

// SPDX-License-Identifier: BSD-3-Clause

package windows

// AgentServiceStatus reports the local Windows service state without attempting
// to start, stop, or reconfigure it.
type AgentServiceStatus struct {
	Installed bool   `json:"installed"`
	State     string `json:"state"`
}

// InspectAgentService reports that Windows Service Control Manager is not
// available on this platform.
func InspectAgentService() (AgentServiceStatus, error) {
	return AgentServiceStatus{State: "unsupported"}, nil
}

// IsElevated reports whether the process has the Windows elevated token.
func IsElevated() bool { return false }
