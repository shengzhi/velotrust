// SPDX-License-Identifier: BSD-3-Clause

package fwmark

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
)

const (
	baseEnv        = "VELOTRUST_FWMARK_BASE"
	defaultBase    = uint32(0x1BD00)
	offsetMask     = uint32(0xFF)
	controlOffset  = uint32(0x00)
	dataInOffset   = uint32(0x10)
	dataOutOffset  = uint32(0x11)
	redirectOffset = uint32(0x20)
	masqOffset     = uint32(0x21)
	masqReturn     = uint32(0x22)
)

var base = loadBase()

var (
	// ControlPlane is applied to sockets that must bypass tunnel routing.
	ControlPlane = base | controlOffset
	// DataPlaneLower and DataPlaneUpper delimit the marks used by routed packets.
	DataPlaneLower = base | dataInOffset
	DataPlaneUpper = base | offsetMask
	// DataPlaneIn and DataPlaneOut identify inbound and outbound tunnel packets.
	DataPlaneIn  = base | dataInOffset
	DataPlaneOut = base | dataOutOffset
	// Redirected, Masquerade, and MasqueradeReturn identify packet processing stages.
	Redirected       = base | redirectOffset
	Masquerade       = base | masqOffset
	MasqueradeReturn = base | masqReturn
)

// ParseBase parses a nonzero fwmark base whose low byte is reserved for individual marks.
func ParseBase(value string) (uint32, error) {
	value = strings.TrimSpace(value)
	parsed, err := strconv.ParseUint(value, 0, 32)
	if err != nil {
		return 0, fmt.Errorf("parse fwmark base: %w", err)
	}
	if parsed == 0 || uint32(parsed)&offsetMask != 0 {
		return 0, fmt.Errorf("fwmark base %#x must be nonzero and leave the low byte free", parsed)
	}
	return uint32(parsed), nil
}

// IsDataPlane reports whether mark is in the configured data-plane range.
func IsDataPlane(mark uint32) bool {
	return mark >= DataPlaneLower && mark <= DataPlaneUpper
}

func loadBase() uint32 {
	value := os.Getenv(baseEnv)
	if value == "" {
		return defaultBase
	}
	parsed, err := ParseBase(value)
	if err != nil {
		slog.Warn("invalid fwmark base; using default", "variable", baseEnv, "error", err)
		return defaultBase
	}
	return parsed
}
