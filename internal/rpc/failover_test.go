// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	assert.Equal(t, 1, client.RotateCount())

	rotated = client.rotateURL()
	assert.True(t, rotated)
	assert.Equal(t, "http://fail1.com", client.HorizonURL)
	assert.Equal(t, 0, client.currIndex)
	assert.Equal(t, 2, client.RotateCount(), "rotate count should reflect two switches")
}

func TestClient_GetHealth_Failover_AllNodesFailedErrorAggregation(t *testing.T) {
	type nodeSpec struct {
		url string
		msg string
	}

	newFailingNode := func(t *testing.T, message string) *httptest.Server {
		t.Helper()
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := GetHealthResponse{
				Jsonrpc: "2.0",
				ID:      1,
				Error: &struct {
					Code    int    `json:"code"`
					Message string `json:"message"`
				}{
					Code:    -32000,
					Message: message,
				},
			}
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(resp); err != nil {
				t.Errorf("failed to encode response in test server: %v", err)
			}
		}))
	}

	server1 := newFailingNode(t, "node-1 failed: backend unavailable")
	defer server1.Close()
	server2 := newFailingNode(t, "node-2 failed: rate limited")
	defer server2.Close()
	server3 := newFailingNode(t, "node-3 failed: invalid request format")
	defer server3.Close()

	nodes := []nodeSpec{
		{url: server1.URL, msg: "node-1 failed: backend unavailable"},
		{url: server2.URL, msg: "node-2 failed: rate limited"},
		{url: server3.URL, msg: "node-3 failed: invalid request format"},
	}

	client, err := NewClient(
		WithNetwork(Testnet),
		WithAltURLs([]string{server1.URL, server2.URL, server3.URL}),
		WithSorobanURL(server1.URL),
	)
	require.NoError(t, err)

	_, err = client.GetHealth(context.Background())
	require.Error(t, err)

	allErr, ok := err.(*AllNodesFailedError)
	require.True(t, ok, "error should be *AllNodesFailedError")
	require.Len(t, allErr.Failures, len(nodes), "failure history should contain one entry per node")

	gotURLs := make(map[string]struct{}, len(allErr.Failures))
	gotMessages := make(map[string]struct{}, len(allErr.Failures))
	for _, failure := range allErr.Failures {
		gotURLs[failure.URL] = struct{}{}
		gotMessages[failure.Reason.Error()] = struct{}{}
	}

	for _, node := range nodes {
		_, hasURL := gotURLs[node.url]
		require.True(t, hasURL, fmt.Sprintf("missing failure URL %q", node.url))

		foundMsg := false
		for gotMsg := range gotMessages {
			if strings.Contains(gotMsg, node.msg) {
				foundMsg = true
				break
			}
		}
		require.True(t, foundMsg, fmt.Sprintf("missing failure message %q", node.msg))
	}

	errMsg := err.Error()
	require.Contains(t, errMsg, "all RPC endpoints failed")
	for _, node := range nodes {
		require.Contains(t, errMsg, node.url)
		require.Contains(t, errMsg, node.msg)
	}
// TestClient_Rotation_SorobanURLSync verifies that after a URL rotation both
// HorizonURL and SorobanURL point to the newly selected node.  Before the
// Protocol V2 standardization, rotateURL contained two dead SorobanURL
// assignments (overwritten by a third) — this test pins the correct invariant.
func TestClient_Rotation_SorobanURLSync(t *testing.T) {
	urls := []string{"http://node1.example.com", "http://node2.example.com"}
	client := NewClientWithURLsOption(urls, Testnet, "")

	client.rotateURL()

	assert.Equal(t, "http://node2.example.com", client.HorizonURL,
		"HorizonURL should reflect the rotated node")
	assert.Equal(t, client.HorizonURL, client.SorobanURL,
		"SorobanURL must stay in sync with HorizonURL after rotation")
}

func TestClient_GetTransaction_Failover_Logic(t *testing.T) {
	server1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server1.Close()

	server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server2.Close()

	client, err := NewClient(
		WithNetwork(Testnet),
		WithAltURLs([]string{server1.URL, server2.URL}),
	)
	require.NoError(t, err)

	_, err = client.GetTransaction(context.Background(), "abc")
	require.Error(t, err)

	allErr, ok := err.(*AllNodesFailedError)
	require.True(t, ok, "error should be *AllNodesFailedError")
	require.Len(t, allErr.Failures, 2, "should have recorded one failure per node")

	gotURLs := make(map[string]struct{}, 2)
	for _, failure := range allErr.Failures {
		gotURLs[failure.URL] = struct{}{}
	}
	_, hasURL1 := gotURLs[server1.URL]
	_, hasURL2 := gotURLs[server2.URL]
	require.True(t, hasURL1, "missing first node URL in failures")
	require.True(t, hasURL2, "missing second node URL in failures")

	require.Contains(t, err.Error(), "all RPC endpoints failed")
}

func TestAllNodesFailedError_Unwrap_ContainsAllPerNodeErrors(t *testing.T) {
	failures := []NodeFailure{
		{URL: "http://node-1.test", Reason: fmt.Errorf("dial timeout")},
		{URL: "http://node-2.test", Reason: fmt.Errorf("rpc method not found")},
		{URL: "http://node-3.test", Reason: fmt.Errorf("503 service unavailable")},
	}

	err := &AllNodesFailedError{Failures: failures}
	unwrapped := err.Unwrap()

	require.Len(t, unwrapped, 3)
	require.EqualError(t, unwrapped[0], "dial timeout")
	require.EqualError(t, unwrapped[1], "rpc method not found")
	require.EqualError(t, unwrapped[2], "503 service unavailable")
}
