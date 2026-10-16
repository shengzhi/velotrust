// SPDX-License-Identifier: BSD-3-Clause

package route

import (
	"fmt"
	"net/netip"
	"sort"
)

// PrefixSetsEqual reports whether two route sets cover the same unique
// prefixes. Prefixes are masked before comparison, so host bits do not cause a
// spurious route change. Invalid prefixes return an error.
func PrefixSetsEqual(a, b []netip.Prefix) (bool, error) {
	left, err := normalizePrefixSet(a)
	if err != nil {
		return false, fmt.Errorf("normalize current route prefixes: %w", err)
	}
	right, err := normalizePrefixSet(b)
	if err != nil {
		return false, fmt.Errorf("normalize desired route prefixes: %w", err)
	}
	if len(left) != len(right) {
		return false, nil
	}
	for i := range left {
		if left[i] != right[i] {
			return false, nil
		}
	}
	return true, nil
}

func normalizePrefixSet(prefixes []netip.Prefix) ([]netip.Prefix, error) {
	set := make(map[netip.Prefix]struct{}, len(prefixes))
	for _, prefix := range prefixes {
		if !prefix.IsValid() {
			return nil, fmt.Errorf("invalid prefix")
		}
		set[prefix.Masked()] = struct{}{}
	}
	result := make([]netip.Prefix, 0, len(set))
	for prefix := range set {
		result = append(result, prefix)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Compare(result[j]) < 0
	})
	return result, nil
}
