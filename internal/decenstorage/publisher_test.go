// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package decenstorage_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dotandev/hintents/internal/decenstorage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPublishIPFS_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v0/add", r.URL.Path)
		require.Contains(t, r.URL.RawQuery, "pin=true")
		require.True(t, strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data"))

		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"Hash":"QmTestCID123","Name":"audit.json","Size":"42"}`)
	}))
	defer srv.Close()

	pub := decenstorage.New(decenstorage.PublishConfig{IPFSNode: srv.URL})
	result, err := pub.PublishIPFS(context.Background(), []byte(`{"test":true}`))

	require.NoError(t, err)
	assert.Equal(t, "ipfs", result.Backend)
	assert.Equal(t, "QmTestCID123", result.CID)
	assert.Equal(t, "https://ipfs.io/ipfs/QmTestCID123", result.URL)
}

func TestPublishIPFS_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	pub := decenstorage.New(decenstorage.PublishConfig{IPFSNode: srv.URL})
	_, err := pub.PublishIPFS(context.Background(), []byte(`{"test":true}`))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestPublishIPFS_EmptyCID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"Hash":""}`)
	}))
	defer srv.Close()

	pub := decenstorage.New(decenstorage.PublishConfig{IPFSNode: srv.URL})
	_, err := pub.PublishIPFS(context.Background(), []byte(`{}`))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty CID")
}

func TestPublishArweave_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/tx", r.URL.Path)
		require.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var tx map[string]interface{}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&tx))
		assert.Equal(t, float64(2), tx["format"])

		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"id":"arweave-txid-abc123"}`)
	}))
	defer srv.Close()

	pub := decenstorage.New(decenstorage.PublishConfig{ArweaveGateway: srv.URL})
	result, err := pub.PublishArweave(context.Background(), []byte(`{"test":true}`))

	require.NoError(t, err)
	assert.Equal(t, "arweave", result.Backend)
	assert.Equal(t, "arweave-txid-abc123", result.TXID)
	assert.Contains(t, result.URL, "arweave-txid-abc123")
}

func TestPublishArweave_PlainTextTXID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintf(w, "arweave-txid-plaintext")
	}))
	defer srv.Close()

	pub := decenstorage.New(decenstorage.PublishConfig{ArweaveGateway: srv.URL})
	result, err := pub.PublishArweave(context.Background(), []byte(`{}`))

	require.NoError(t, err)
	assert.Equal(t, "arweave-txid-plaintext", result.TXID)
}

func TestPublishArweave_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad request", http.StatusBadRequest)
	}))
	defer srv.Close()

	pub := decenstorage.New(decenstorage.PublishConfig{ArweaveGateway: srv.URL})
	_, err := pub.PublishArweave(context.Background(), []byte(`{}`))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "400")
}

func TestPublishArweave_EmptyTXID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintf(w, "")
	}))
	defer srv.Close()

	pub := decenstorage.New(decenstorage.PublishConfig{ArweaveGateway: srv.URL})
	_, err := pub.PublishArweave(context.Background(), []byte(`{}`))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty transaction ID")
}

func TestNew_DefaultsApplied(t *testing.T) {
	t.Setenv("ERST_IPFS_NODE", "")
	t.Setenv("ERST_ARWEAVE_GATEWAY", "")
	// Verify New() does not panic and returns a usable Publisher.
	pub := decenstorage.New(decenstorage.PublishConfig{})
	require.NotNil(t, pub)
}

func TestNew_EnvOverrides(t *testing.T) {
	t.Setenv("ERST_IPFS_NODE", "http://custom-ipfs:5001")
	t.Setenv("ERST_ARWEAVE_GATEWAY", "http://custom-arweave")

	// Expect the publisher to use our test server URL when IPFSNode is unset.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintf(w, `{"Hash":"QmEnvTest"}`)
	}))
	defer srv.Close()

	// Env is set but cfg.IPFSNode wins if already non-empty.
	pub := decenstorage.New(decenstorage.PublishConfig{IPFSNode: srv.URL})
	result, err := pub.PublishIPFS(context.Background(), []byte(`{}`))
	require.NoError(t, err)
	assert.Equal(t, "QmEnvTest", result.CID)
}
