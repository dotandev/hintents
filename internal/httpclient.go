// Copyright (c) Hintents Authors.
// SPDX-License-Identifier: Apache-2.0

package internal

import "net/http"

// HTTPClient abstracts http.Client for testability
type HTTPClient interface {
    Do(req *http.Request) (*http.Response, error)
}