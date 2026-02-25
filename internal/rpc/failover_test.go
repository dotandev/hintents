// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClient_Rotation(t *testing.T) {
	urls := []string{"http://fail1.com", "http://success2.com"}
	client := NewClientWithURLsOption(urls, Testnet, "")

	assert.Equal(t, "http://fail1.com", client.HorizonURL)
	assert.Equal(t, 0, client.currIndex)

	rotated := client.rotateURL()
	assert.True(t, rotated)
	assert.Equal(t, "http://success2.com", client.HorizonURL)
	assert.Equal(t, 1, client.currIndex)

	rotated = client.rotateURL()
	assert.True(t, rotated)
	assert.Equal(t, "http://fail1.com", client.HorizonURL) // Wraps around
	assert.Equal(t, 0, client.currIndex)
}

func TestClient_GetTransaction_Failover_Logic(t *testing.T) {
	// This test verifies that GetTransaction calls rotateURL and retries
	// We'll use a subclass to intercept rotateURL for testing if needed,
	// or just rely on the fact that GetTransaction uses AltURLs loop.

	// Since rotateURL recreates the horizon client, we'll just test the loop logic
	// by checking that it returns an error after trying all URLs if they all fail.

	urls := []string{"http://fail1.com", "http://fail2.com"}
	client := NewClientWithURLsOption(urls, Testnet, "")

	ctx := context.Background()
	_, err := client.GetTransaction(ctx, "abc")

	assert.Error(t, err)
	fallbackErr, ok := err.(*AllNodesFailedError)
	assert.True(t, ok, "Error should be of type *AllNodesFailedError")
	assert.Equal(t, 2, len(fallbackErr.Failures), "Should have recorded 2 failures")
	assert.Contains(t, err.Error(), "all RPC endpoints failed")
}
