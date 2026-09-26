package rpc

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

// EventsHandler handles the /events endpoint.
// It implements pagination via a ?cursor= parameter based on the ledger sequence
// and limits results to a maximum of 100 per page to prevent browser crashes.
func (c *Client) EventsHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	// Default limit to 100 maximum
	limit := 100
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}
	
	// Read cursor (ledger sequence)
	cursor := r.URL.Query().Get("cursor")
	startLedger := uint32(0)
	
	if cursor != "" {
		if val, err := strconv.ParseUint(cursor, 10, 32); err == nil {
			startLedger = uint32(val)
		}
	} else {
		// Default to latest ledger if no cursor is provided
		latest, err := c.GetLatestLedgerSequence(ctx)
		if err == nil && latest > 0 {
			if latest > 100 {
				startLedger = uint32(latest - 100)
			} else {
				startLedger = 1
			}
		}
	}

	params := map[string]interface{}{
		"startLedger": startLedger,
		"pagination": map[string]interface{}{
			"limit": limit,
		},
	}
	
	if cursor != "" {
		params["pagination"].(map[string]interface{})["cursor"] = cursor
	}

	reqBody := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "getEvents",
		"params":  params,
	}
	
	var rpcResp interface{}
	err := c.postRequest(ctx, reqBody, &rpcResp)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to fetch events: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rpcResp)
}
