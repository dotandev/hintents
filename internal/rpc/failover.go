// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"fmt"
	"strings"
	"time"

	"github.com/dotandev/hintents/internal/logger"
	"github.com/stellar/go-stellar-sdk/clients/horizonclient"
)

// NodeFailure records a failure for a specific RPC URL
type NodeFailure struct {
	URL    string
	Reason error
}

// AllNodesFailedError represents a failure after exhausting all RPC endpoints
type AllNodesFailedError struct {
	Failures []NodeFailure
}

func (e *AllNodesFailedError) Error() string {
	var reasons []string
	for _, f := range e.Failures {
		reasons = append(reasons, fmt.Sprintf("%s: %v", f.URL, f.Reason))
	}
	return fmt.Sprintf("all RPC endpoints failed: [%s]", strings.Join(reasons, ", "))
}

// Unwrap returns all per-node errors so errors.Is and errors.As can traverse them.
func (e *AllNodesFailedError) Unwrap() []error {
	errs := make([]error, len(e.Failures))
	for i, f := range e.Failures {
		errs[i] = f.Reason
	}
	return errs
}

// endpointAttempts returns how many attempts should be made across endpoint lists.
func (c *Client) endpointAttempts() int {
	if len(c.AltURLs) == 0 {
		return 1
	}
	return len(c.AltURLs)
}

// isHealthy checks if an endpoint is currently healthy or if circuit is open.
// This is a best-effort check — there is an intentional TOCTOU window between
// this call and the subsequent http.Do; no lock is held across both operations
// because doing so would risk deadlocks with rotateURL. The circuit breaker is
// an optimistic fast-path, not a hard guarantee.
func (c *Client) isHealthy(url string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.isHealthyLocked(url)
}

func (c *Client) isHealthyLocked(url string) bool {
	fails := c.failures[url]
	if fails < 5 {
		return true
	}
	last := c.lastFailure[url]
	// Circuit opens for 60 seconds
	if time.Since(last) > 60*time.Second {
		return true
	}
	return false
}

func (c *Client) markFailure(url string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.markFailureLocked(url)
}

func (c *Client) markSuccess(url string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.markSuccessLocked(url)
}

func (c *Client) markFailureLocked(url string) {
	if c.failures == nil {
		c.failures = make(map[string]int)
	}
	if c.lastFailure == nil {
		c.lastFailure = make(map[string]time.Time)
	}
	c.failures[url]++
	c.lastFailure[url] = time.Now()
}

func (c *Client) markSuccessLocked(url string) {
	if c.failures == nil {
		c.failures = make(map[string]int)
	}
	if c.lastFailure == nil {
		c.lastFailure = make(map[string]time.Time)
	}
	delete(c.failures, url)
	delete(c.lastFailure, url)
}

// rotateURL switches to the next available provider URL, preferring healthier nodes.
func (c *Client) rotateURL() bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.AltURLs) <= 1 {
		return false
	}

	currentURL := c.AltURLs[c.currIndex]

	// Build list of healthy candidates excluding current URL.
	// Build candidate list excluding current URL.
	candidates := make([]string, 0, len(c.AltURLs)-1)
	for _, url := range c.AltURLs {
		if url != currentURL && c.isHealthyLocked(url) {
			candidates = append(candidates, url)
		}
	}

	// If we have health telemetry, prefer the healthiest candidate.
	if len(candidates) > 0 && c.healthCollector != nil {
		bestURL := c.healthCollector.GetHealthiestURL(candidates)
		if bestURL != "" {
			for i, url := range c.AltURLs {
				if url == bestURL {
					c.currIndex = i
					break
				}
			}
		} else {
			c.currIndex = (c.currIndex + 1) % len(c.AltURLs)
		}
	} else {
		// Fall back to round-robin, preferring healthy endpoints when possible.
		for i := 0; i < len(c.AltURLs); i++ {
		c.currIndex = (c.currIndex + 1) % len(c.AltURLs)
		url := c.AltURLs[c.currIndex]
		if c.isHealthyLocked(url) {
			break
		}
	}

	// If no healthy candidate is available, fall back to simple round-robin.
	if len(candidates) == 0 {
		c.currIndex = (c.currIndex + 1) % len(c.AltURLs)
	} else if c.healthCollector != nil {
		bestURL := c.healthCollector.GetHealthiestURL(candidates)
		if bestURL != "" {
			for i, url := range c.AltURLs {
				if url == bestURL {
					c.currIndex = i
					break
				}
			}
		} else {
			c.currIndex = (c.currIndex + 1) % len(c.AltURLs)
		}
	} else {
		c.currIndex = (c.currIndex + 1) % len(c.AltURLs)
	}
	}

	c.HorizonURL = c.AltURLs[c.currIndex]
	// Keep SorobanURL in sync with HorizonURL so that health checks and Soroban RPC
	// calls reflect the failover.
	c.SorobanURL = c.HorizonURL

	httpClient := c.httpClient
	if httpClient == nil {
		httpClient = createHTTPClient(c.token, defaultHTTPTimeout, c.middlewares...)
	}

	c.Horizon = &horizonclient.Client{
		HorizonURL: c.HorizonURL,
		HTTP:       httpClient,
	}

	logger.Logger.Warn("RPC failover triggered", "new_url", c.HorizonURL)
	// increment counter under the same lock so readers get a consistent view
	c.rotateCount++
	return true
}

// RotateCount returns the number of times the client has switched
// to a different Horizon URL via rotateURL.  It is safe for concurrent
// use.
func (c *Client) RotateCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.rotateCount
}
