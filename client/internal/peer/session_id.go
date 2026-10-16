// SPDX-License-Identifier: BSD-3-Clause

package peer

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
)

const sessionIDSize = 5

// SessionID distinguishes successive peer negotiation sessions.
type SessionID string

// NewSessionID creates a cryptographically random session identifier.
func NewSessionID() (SessionID, error) {
	data := make([]byte, sessionIDSize)
	if _, err := io.ReadFull(rand.Reader, data); err != nil {
		return "", fmt.Errorf("generate peer session ID: %w", err)
	}
	return SessionID(hex.EncodeToString(data)), nil
}

// SessionIDFromBytes converts the wire representation to its canonical hex form.
func SessionIDFromBytes(data []byte) (SessionID, error) {
	if len(data) != sessionIDSize {
		return "", fmt.Errorf("invalid peer session ID length: %d", len(data))
	}
	return SessionID(hex.EncodeToString(data)), nil
}

// Bytes validates and decodes the session identifier.
func (id SessionID) Bytes() ([]byte, error) {
	if len(id) == 0 {
		return nil, fmt.Errorf("peer session ID is empty")
	}
	data, err := hex.DecodeString(string(id))
	if err != nil {
		return nil, fmt.Errorf("invalid peer session ID encoding: %w", err)
	}
	if len(data) != sessionIDSize {
		return nil, fmt.Errorf("invalid peer session ID length: expected %d bytes, got %d", sessionIDSize, len(data))
	}
	return data, nil
}

func (id SessionID) String() string { return string(id) }
