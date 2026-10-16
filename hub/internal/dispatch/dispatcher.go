package dispatch

import (
	"context"

	hubv1 "velotrust/shared/gen/go/hub/v1"
)

// Dispatcher routes control messages to the HUB instance that owns a node's
// active stream. Implementations must not report successful delivery before
// the message is accepted by that stream or a durable queue.
type Dispatcher interface {
	Send(context.Context, string, *hubv1.HubMessage) error
}
