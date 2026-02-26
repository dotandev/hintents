// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

// Package decenstorage provides clients for pushing signed audit-trail payloads
// to decentralised storage networks.
//
// Two backends are supported:
//
//   - IPFS: the payload is pinned via the Kubo HTTP RPC API
//     (default gateway: http://localhost:5001).  The API endpoint is
//     configurable via PublishConfig.IPFSNode or the ERST_IPFS_NODE
//     environment variable.
//
//   - Arweave: the payload is uploaded via the Arweave HTTP gateway
//     (default: https://arweave.net).  A wallet JWK and gateway URL are
//     configurable via PublishConfig or the ERST_ARWEAVE_WALLET /
//     ERST_ARWEAVE_GATEWAY environment variables.
//
// Neither backend is contacted unless the caller explicitly requests it.
package decenstorage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	defaultIPFSNode      = "http://localhost:5001"
	defaultArweaveGW     = "https://arweave.net"
	defaultTimeout       = 30 * time.Second
	envIPFSNode          = "ERST_IPFS_NODE"
	envArweaveGateway    = "ERST_ARWEAVE_GATEWAY"
	envArweaveWallet     = "ERST_ARWEAVE_WALLET"
)

// PublishConfig carries optional overrides for both backends.
// Zero-value fields fall back to environment variables and then built-in
// defaults, so callers that only set the boolean flags need no extra wiring.
type PublishConfig struct {
	// IPFSNode is the base URL of a Kubo-compatible IPFS HTTP RPC node.
	// Example: "http://localhost:5001".
	IPFSNode string

	// ArweaveGateway is the base URL of the Arweave HTTP gateway.
	// Example: "https://arweave.net".
	ArweaveGateway string

	// ArweaveWallet is the path to an Arweave JWK wallet file used to sign
	// data transactions.  When empty the raw payload is posted unsigned
	// (only valid against a local Arweave test node).
	ArweaveWallet string
}

// Result carries the storage reference returned by each backend after a
// successful upload.
type Result struct {
	// Backend is either "ipfs" or "arweave".
	Backend string `json:"backend"`

	// CID is the IPFS content identifier (only set for the IPFS backend).
	CID string `json:"cid,omitempty"`

	// TXID is the Arweave transaction identifier (only set for the Arweave backend).
	TXID string `json:"txid,omitempty"`

	// URL is a human-readable public gateway URL for the stored content.
	URL string `json:"url"`
}

// Publisher dispatches a signed audit payload to one or more decentralised
// storage backends.
type Publisher struct {
	cfg    PublishConfig
	client *http.Client
}

// New returns a Publisher whose HTTP behaviour is governed by cfg.
// Missing cfg fields are resolved from environment variables.
func New(cfg PublishConfig) *Publisher {
	if node := os.Getenv(envIPFSNode); node != "" && cfg.IPFSNode == "" {
		cfg.IPFSNode = node
	}
	if cfg.IPFSNode == "" {
		cfg.IPFSNode = defaultIPFSNode
	}

	if gw := os.Getenv(envArweaveGateway); gw != "" && cfg.ArweaveGateway == "" {
		cfg.ArweaveGateway = gw
	}
	if cfg.ArweaveGateway == "" {
		cfg.ArweaveGateway = defaultArweaveGW
	}

	if wallet := os.Getenv(envArweaveWallet); wallet != "" && cfg.ArweaveWallet == "" {
		cfg.ArweaveWallet = wallet
	}

	return &Publisher{
		cfg: cfg,
		client: &http.Client{
			Timeout: defaultTimeout,
		},
	}
}

// PublishIPFS pins payload to IPFS via the Kubo HTTP RPC add endpoint and
// returns the resulting CID.
func (p *Publisher) PublishIPFS(ctx context.Context, payload []byte) (Result, error) {
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)

	fw, err := mw.CreateFormFile("file", "audit.json")
	if err != nil {
		return Result{}, fmt.Errorf("decenstorage: failed to create form file: %w", err)
	}
	if _, err := fw.Write(payload); err != nil {
		return Result{}, fmt.Errorf("decenstorage: failed to write payload: %w", err)
	}
	if err := mw.Close(); err != nil {
		return Result{}, fmt.Errorf("decenstorage: failed to close multipart writer: %w", err)
	}

	url := strings.TrimRight(p.cfg.IPFSNode, "/") + "/api/v0/add?pin=true"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &body)
	if err != nil {
		return Result{}, fmt.Errorf("decenstorage: failed to build IPFS request: %w", err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())

	resp, err := p.client.Do(req)
	if err != nil {
		return Result{}, fmt.Errorf("decenstorage: IPFS request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return Result{}, fmt.Errorf("decenstorage: IPFS node returned %d", resp.StatusCode)
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return Result{}, fmt.Errorf("decenstorage: failed to read IPFS response: %w", err)
	}

	var addResp struct {
		Hash string `json:"Hash"`
	}
	if err := json.Unmarshal(raw, &addResp); err != nil {
		return Result{}, fmt.Errorf("decenstorage: failed to parse IPFS response: %w", err)
	}
	if addResp.Hash == "" {
		return Result{}, fmt.Errorf("decenstorage: IPFS returned empty CID")
	}

	return Result{
		Backend: "ipfs",
		CID:     addResp.Hash,
		URL:     "https://ipfs.io/ipfs/" + addResp.Hash,
	}, nil
}

// arweaveTx is a minimal representation of an Arweave data transaction.
// Full signing requires reading the wallet JWK and computing the data root;
// for now we build a best-effort unsigned transaction accepted by local nodes
// and well-funded gateway POST endpoints (e.g. irys.xyz).
type arweaveTx struct {
	Format   int               `json:"format"`
	LastTx   string            `json:"last_tx"`
	Owner    string            `json:"owner"`
	Tags     []arweaveTag      `json:"tags"`
	Target   string            `json:"target"`
	Quantity string            `json:"quantity"`
	Data     []byte            `json:"data"`
	Reward   string            `json:"reward"`
	Signature string           `json:"signature"`
	ID       string            `json:"id"`
}

type arweaveTag struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// PublishArweave posts payload to Arweave and returns the transaction ID.
//
// When ArweaveWallet is set it is read as a JWK file and the owner field is
// populated.  Full cryptographic signing of Arweave transactions requires RSA
// PSS primitives that are outside the scope of this package; callers that need
// production Arweave support should use a dedicated Arweave SDK.  This
// implementation produces a valid unsigned transaction accepted by local
// ArLocal instances and Bundlr/Irys upload endpoints.
func (p *Publisher) PublishArweave(ctx context.Context, payload []byte) (Result, error) {
	owner := ""
	if p.cfg.ArweaveWallet != "" {
		walletBytes, err := os.ReadFile(p.cfg.ArweaveWallet)
		if err != nil {
			return Result{}, fmt.Errorf("decenstorage: failed to read Arweave wallet: %w", err)
		}
		var jwk map[string]interface{}
		if err := json.Unmarshal(walletBytes, &jwk); err != nil {
			return Result{}, fmt.Errorf("decenstorage: failed to parse Arweave wallet: %w", err)
		}
		if n, ok := jwk["n"].(string); ok {
			owner = n
		}
	}

	tx := arweaveTx{
		Format:   2,
		LastTx:   "",
		Owner:    owner,
		Target:   "",
		Quantity: "0",
		Data:     payload,
		Reward:   "0",
		Signature: "",
		Tags: []arweaveTag{
			{Name: "Content-Type", Value: "application/json"},
			{Name: "App-Name", Value: "erst"},
			{Name: "App-Version", Value: "1.0.0"},
		},
	}

	txBytes, err := json.Marshal(tx)
	if err != nil {
		return Result{}, fmt.Errorf("decenstorage: failed to marshal Arweave transaction: %w", err)
	}

	url := strings.TrimRight(p.cfg.ArweaveGateway, "/") + "/tx"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(txBytes))
	if err != nil {
		return Result{}, fmt.Errorf("decenstorage: failed to build Arweave request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return Result{}, fmt.Errorf("decenstorage: Arweave request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return Result{}, fmt.Errorf("decenstorage: Arweave gateway returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return Result{}, fmt.Errorf("decenstorage: failed to read Arweave response: %w", err)
	}

	// The Arweave gateway returns the TXID in the response body as plain text
	// or as a JSON object {"id":"..."}.
	txid := strings.TrimSpace(string(raw))
	var jsonResp map[string]string
	if err := json.Unmarshal(raw, &jsonResp); err == nil {
		if id, ok := jsonResp["id"]; ok && id != "" {
			txid = id
		}
	}

	if txid == "" {
		return Result{}, fmt.Errorf("decenstorage: Arweave returned empty transaction ID")
	}

	return Result{
		Backend: "arweave",
		TXID:    txid,
		URL:     strings.TrimRight(p.cfg.ArweaveGateway, "/") + "/" + txid,
	}, nil
}
