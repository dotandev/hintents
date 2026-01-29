// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/dotandev/hintents/internal/errors"
	"github.com/dotandev/hintents/internal/logger"
	"github.com/stellar/go/clients/horizonclient"
)

// Network types for Stellar
type Network string

const (
	Testnet   Network = "testnet"
	Mainnet   Network = "mainnet"
	Futurenet Network = "futurenet"
)

// Horizon URLs for each network
const (
	TestnetHorizonURL   = "https://horizon-testnet.stellar.org/"
	MainnetHorizonURL   = "https://horizon.stellar.org/"
	FuturenetHorizonURL = "https://horizon-futurenet.stellar.org/"
)

// Client handles interactions with the Stellar Network
type Client struct {
	Horizon horizonclient.ClientInterface
	Network Network
	RPCURL  string
	HTTP    *http.Client
}

// NewClient creates a new RPC client with the specified network
// If network is empty, defaults to Mainnet
func NewClient(net Network) *Client {
	if net == "" {
		net = Mainnet
	}

	var horizonClient *horizonclient.Client

	switch net {
	case Testnet:
		horizonClient = horizonclient.DefaultTestNetClient
	case Futurenet:
		// Create a futurenet client (not available as default)
		horizonClient = &horizonclient.Client{
			HorizonURL: FuturenetHorizonURL,
			HTTP:       http.DefaultClient,
		}
	case Mainnet:
		fallthrough
	default:
		horizonClient = horizonclient.DefaultPublicNetClient
	}

	return &Client{
		Horizon: horizonClient,
		Network: net,
		RPCURL:  horizonClient.HorizonURL,
		HTTP: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// NewClientWithURL creates a new RPC client with a custom Horizon URL
func NewClientWithURL(url string, net Network) *Client {
	horizonClient := &horizonclient.Client{
		HorizonURL: url,
		HTTP:       http.DefaultClient,
	}

	return &Client{
		Horizon: horizonClient,
		Network: net,
		RPCURL:  url,
		HTTP: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// GetTransaction fetches the transaction details and full XDR data
func (c *Client) GetTransaction(ctx context.Context, hash string) (*TransactionResponse, error) {
	logger.Logger.Debug("Fetching transaction details", "hash", hash)

	tx, err := c.Horizon.TransactionDetail(hash)
	if err != nil {
		logger.Logger.Error("Failed to fetch transaction", "hash", hash, "error", err)
		return nil, errors.WrapTransactionNotFound(err)
	}

	logger.Logger.Info("Transaction fetched successfully", "hash", hash, "envelope_size", len(tx.EnvelopeXdr))

	return parseTransactionResponse(tx), nil
}

type jsonRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int64       `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type jsonRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *jsonRPCError   `json:"error,omitempty"`
}

type jsonRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type getLedgerEntriesParams struct {
	Keys []string `json:"keys"`
}

type getLedgerEntriesResult struct {
	Entries []struct {
		Key string `json:"key"`
		XDR string `json:"xdr"`
	} `json:"entries"`
}

func (c *Client) doJSONRPC(ctx context.Context, method string, params interface{}, out interface{}) error {
	if c.RPCURL == "" {
		return fmt.Errorf("rpc url is empty")
	}
	if c.HTTP == nil {
		c.HTTP = &http.Client{Timeout: 15 * time.Second}
	}

	reqBody, err := json.Marshal(jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      time.Now().UnixNano(),
		Method:  method,
		Params:  params,
	})
	if err != nil {
		return fmt.Errorf("marshal json-rpc request: %w", err)
	}

	ctxReq := ctx
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctxReq, cancel = context.WithTimeout(ctx, 15*time.Second)
		defer cancel()
	}

	httpReq, err := http.NewRequestWithContext(ctxReq, http.MethodPost, c.RPCURL, bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("create json-rpc request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := c.HTTP.Do(httpReq)
	if err != nil {
		return fmt.Errorf("json-rpc http request: %w", err)
	}
	defer httpResp.Body.Close()

	respBytes, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return fmt.Errorf("read json-rpc response: %w", err)
	}
	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		return fmt.Errorf("json-rpc http status %d: %s", httpResp.StatusCode, string(respBytes))
	}

	var rpcResp jsonRPCResponse
	if err := json.Unmarshal(respBytes, &rpcResp); err != nil {
		return fmt.Errorf("unmarshal json-rpc envelope: %w", err)
	}
	if rpcResp.Error != nil {
		return fmt.Errorf("json-rpc error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(rpcResp.Result, out); err != nil {
		return fmt.Errorf("unmarshal json-rpc result: %w", err)
	}
	return nil
}

func chunkStrings(in []string, size int) [][]string {
	if size <= 0 {
		size = 1
	}
	if len(in) == 0 {
		return nil
	}
	chunks := make([][]string, 0, (len(in)+size-1)/size)
	for i := 0; i < len(in); i += size {
		end := i + size
		if end > len(in) {
			end = len(in)
		}
		chunks = append(chunks, in[i:end])
	}
	return chunks
}

// GetLedgerEntries fetches multiple ledger entries by their base64-encoded LedgerKey XDR.
// Returns a map of key (base64 LedgerKey XDR) to entry (base64 LedgerEntry XDR).
func (c *Client) GetLedgerEntries(ctx context.Context, keys []string) (map[string]string, error) {
	result := make(map[string]string, len(keys))
	if len(keys) == 0 {
		return result, nil
	}

	const chunkSize = 50
	const maxConcurrent = 6

	chunks := chunkStrings(keys, chunkSize)
	sem := make(chan struct{}, maxConcurrent)

	var (
		mu     sync.Mutex
		wg     sync.WaitGroup
		errMu  sync.Mutex
		firstE error
	)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	for _, chunk := range chunks {
		wg.Add(1)
		chunk := chunk
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}

			var res getLedgerEntriesResult
			err := c.doJSONRPC(ctx, "getLedgerEntries", getLedgerEntriesParams{Keys: chunk}, &res)
			if err != nil {
				errMu.Lock()
				if firstE == nil {
					firstE = err
					cancel()
				}
				errMu.Unlock()
				return
			}

			mu.Lock()
			for _, e := range res.Entries {
				if e.Key == "" {
					continue
				}
				result[e.Key] = e.XDR
			}
			mu.Unlock()
		}()
	}

	wg.Wait()
	if firstE != nil {
		return nil, firstE
	}
	return result, nil
}
