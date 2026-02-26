// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package offline

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/dotandev/hintents/internal/errors"
	"github.com/dotandev/hintents/internal/logger"
)

// SubmitRequest is the JSON-RPC request body for sendTransaction.
type SubmitRequest struct {
	Jsonrpc string        `json:"jsonrpc"`
	ID      int           `json:"id"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
}

// SubmitResponse is the JSON-RPC response from sendTransaction.
type SubmitResponse struct {
	Jsonrpc string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Result  struct {
		Status string `json:"status"`
		Hash   string `json:"hash"`
	} `json:"result"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// SubmitSignedEnvelope sends the signed TransactionEnvelope XDR to the Soroban
// RPC endpoint via sendTransaction.
func SubmitSignedEnvelope(ctx context.Context, sorobanURL, envelopeXDR string) (*SubmitResponse, error) {
	if sorobanURL == "" {
		return nil, errors.WrapValidationError("soroban RPC URL is required for submission")
	}

	if envelopeXDR == "" {
		return nil, errors.WrapValidationError("envelope XDR is empty")
	}

	reqBody := SubmitRequest{
		Jsonrpc: "2.0",
		ID:      1,
		Method:  "sendTransaction",
		Params:  []interface{}{envelopeXDR},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, errors.WrapMarshalFailed(err)
	}

	logger.Logger.Debug("Submitting signed transaction", "url", sorobanURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, sorobanURL, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return nil, errors.WrapRPCConnectionFailed(err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, errors.WrapRPCConnectionFailed(err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.WrapUnmarshalFailed(err, "body read error")
	}

	var rpcResp SubmitResponse
	if err := json.Unmarshal(respBytes, &rpcResp); err != nil {
		return nil, errors.WrapUnmarshalFailed(err, string(respBytes))
	}

	if rpcResp.Error != nil {
		return nil, errors.WrapRPCError(sorobanURL, rpcResp.Error.Message, rpcResp.Error.Code)
	}

	return &rpcResp, nil
}

// SorobanURLForNetwork returns the default Soroban RPC URL for the given network name.
func SorobanURLForNetwork(network string) (string, error) {
	switch network {
	case "testnet":
		return "https://soroban-testnet.stellar.org", nil
	case "mainnet":
		return "https://mainnet.stellar.validationcloud.io/v1/soroban-rpc-demo", nil
	case "futurenet":
		return "https://rpc-futurenet.stellar.org", nil
	default:
		return "", errors.WrapInvalidNetwork(fmt.Sprintf("%s (expected testnet, mainnet, or futurenet)", network))
	}
}
