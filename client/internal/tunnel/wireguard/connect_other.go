//go:build !windows

// SPDX-License-Identifier: BSD-3-Clause

package wireguard

import (
	"context"
	"errors"
	"io"

	clientconfig "velotrust/client/internal/config"
)

func Connect(context.Context, string, clientconfig.Tunnel, io.Writer) error {
	return errors.New("live client tunnel connection is supported on Windows only")
}
