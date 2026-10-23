package postgres

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

const maxAuditActorLength = 64

// auditActor identifies HUB-originated work. Human-triggered operations use
// the human username, while authenticated nodes use nodeAuditActor.
func auditActor(instanceID string) string {
	return boundedAuditPrincipal("hub:", instanceID)
}

func nodeAuditActor(nodeType, nodeCode string) string {
	var prefix string
	switch nodeType {
	case "gateway":
		prefix = "gateway:"
	case "client_agent":
		prefix = "client:"
	default:
		return ""
	}
	return boundedAuditPrincipal(prefix, nodeCode)
}

func boundedAuditPrincipal(prefix, identity string) string {
	identity = strings.TrimSpace(identity)
	if identity == "" {
		return ""
	}
	principal := prefix + identity
	if len(principal) <= maxAuditActorLength {
		return principal
	}
	digest := sha256.Sum256([]byte(identity))
	encoded := hex.EncodeToString(digest[:])
	return prefix + encoded[:maxAuditActorLength-len(prefix)]
}
