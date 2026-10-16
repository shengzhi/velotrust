//go:build windows

// SPDX-License-Identifier: BSD-3-Clause

package windows

import (
	"errors"
	"fmt"
	"syscall"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
)

const AgentServiceName = "VeloTrustClientAgent"

// AgentServiceStatus reports the local Windows service state without attempting
// to start, stop, or reconfigure it.
type AgentServiceStatus struct {
	Installed bool   `json:"installed"`
	State     string `json:"state"`
}

// InspectAgentService queries the Windows Service Control Manager.
func InspectAgentService() (AgentServiceStatus, error) {
	manager, err := windows.OpenSCManager(nil, nil, windows.SC_MANAGER_CONNECT)
	if err != nil {
		return AgentServiceStatus{State: "unknown"}, fmt.Errorf("connect to Windows Service Control Manager: %w", err)
	}
	defer windows.CloseServiceHandle(manager)

	serviceName, err := syscall.UTF16PtrFromString(AgentServiceName)
	if err != nil {
		return AgentServiceStatus{State: "unknown"}, fmt.Errorf("encode client agent service name: %w", err)
	}
	service, err := windows.OpenService(manager, serviceName, windows.SERVICE_QUERY_STATUS)
	if err != nil {
		if errors.Is(err, syscall.Errno(windows.ERROR_SERVICE_DOES_NOT_EXIST)) {
			return AgentServiceStatus{Installed: false, State: "not-installed"}, nil
		}
		return AgentServiceStatus{State: "unknown"}, fmt.Errorf("inspect client agent service: %w", err)
	}
	defer windows.CloseServiceHandle(service)

	var status windows.SERVICE_STATUS
	if err := windows.QueryServiceStatus(service, &status); err != nil {
		return AgentServiceStatus{Installed: true, State: "unknown"}, fmt.Errorf("query client agent service: %w", err)
	}
	state := svc.State(status.CurrentState)
	return AgentServiceStatus{Installed: true, State: serviceStateName(state)}, nil
}

// IsElevated reports whether this process has an elevated Windows token.
func IsElevated() bool {
	return windows.GetCurrentProcessToken().IsElevated()
}

func serviceStateName(state svc.State) string {
	switch state {
	case svc.Stopped:
		return "stopped"
	case svc.StartPending:
		return "start-pending"
	case svc.StopPending:
		return "stop-pending"
	case svc.Running:
		return "running"
	case svc.ContinuePending:
		return "continue-pending"
	case svc.PausePending:
		return "pause-pending"
	case svc.Paused:
		return "paused"
	default:
		return "unknown"
	}
}
