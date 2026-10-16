// SPDX-License-Identifier: BSD-3-Clause

package peer

import "fmt"

// ConnectionPriority ranks available peer transports from least to most preferred.
type ConnectionPriority int

const (
	PriorityNone ConnectionPriority = iota
	PriorityRelay
	PriorityRelayTurn
	PriorityDirect
)

func (p ConnectionPriority) String() string {
	switch p {
	case PriorityNone:
		return "None"
	case PriorityRelay:
		return "Relay"
	case PriorityRelayTurn:
		return "RelayTurn"
	case PriorityDirect:
		return "Direct"
	default:
		return fmt.Sprintf("ConnectionPriority(%d)", p)
	}
}

// ConnectionStatus describes whether the required path to a peer is usable.
type ConnectionStatus int

const (
	ConnectionDisconnected ConnectionStatus = iota
	ConnectionPartiallyConnected
	ConnectionConnected
)

func (s ConnectionStatus) String() string {
	switch s {
	case ConnectionDisconnected:
		return "Disconnected"
	case ConnectionPartiallyConnected:
		return "PartiallyConnected"
	case ConnectionConnected:
		return "Connected"
	default:
		return fmt.Sprintf("ConnectionStatus(%d)", s)
	}
}

// ConnectionStatusInput is the transport snapshot used for peer status decisions.
// Missing or unavailable transports are represented as false and never count as
// an established path.
type ConnectionStatusInput struct {
	ForceRelay           bool
	PeerUsesRelay        bool
	RelayConnected       bool
	RemoteSupportsDirect bool
	DirectWorkerReady    bool
	DirectConnecting     bool
	DirectNegotiating    bool
}

// EvaluateConnectionStatus returns the most conservative status supported by
// the current relay and direct-connection state.
func EvaluateConnectionStatus(in ConnectionStatusInput) ConnectionStatus {
	relayUp := in.PeerUsesRelay && in.RelayConnected
	if in.ForceRelay || !in.RemoteSupportsDirect || !in.DirectWorkerReady {
		if relayUp {
			return ConnectionConnected
		}
		return ConnectionDisconnected
	}

	directUp := in.DirectConnecting || in.DirectNegotiating
	if directUp && (!in.PeerUsesRelay || in.RelayConnected) {
		return ConnectionConnected
	}
	if relayUp {
		return ConnectionPartiallyConnected
	}
	return ConnectionDisconnected
}
