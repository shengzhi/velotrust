// SPDX-License-Identifier: BSD-3-Clause

package route

import "strings"

const haSeparator = "|"

// NetID identifies the network shared by a set of high availability routes.
type NetID string

// HAUniqueID groups routes that provide the same network through different peers.
type HAUniqueID string

func (id HAUniqueID) String() string {
	return string(id)
}

// NetID returns the network identifier preceding the final separator.
func (id HAUniqueID) NetID() NetID {
	if index := strings.LastIndex(string(id), haSeparator); index >= 0 {
		return NetID(id[:index])
	}
	return NetID(id)
}
