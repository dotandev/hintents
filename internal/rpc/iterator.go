// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package rpc

import "fmt"

const horizonPageMaxLimit = 200

func normalizePageSize(limit int) int {
	if limit <= 0 {
		return horizonPageMaxLimit
	}
	if limit > horizonPageMaxLimit {
		return horizonPageMaxLimit
	}
	return limit
}

type pageIterator[P any, R any] struct {
	first   func() (P, error)
	next    func(P) (P, error)
	records func(P) []R
	max     int
}

func (it pageIterator[P, R]) collect() ([]R, error) {
	page, err := it.first()
	if err != nil {
		return nil, err
	}

	out := make([]R, 0)
	for {
		rows := it.records(page)
		if len(rows) == 0 {
			return out, nil
		}

		if it.max > 0 {
			remaining := it.max - len(out)
			if remaining <= 0 {
				return out, nil
			}
			if len(rows) > remaining {
				rows = rows[:remaining]
			}
		}
		out = append(out, rows...)

		if it.max > 0 && len(out) >= it.max {
			return out, nil
		}

		page, err = it.next(page)
		if err != nil {
			return out, fmt.Errorf("fetch next page: %w", err)
		}
	}
}
