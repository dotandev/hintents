// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"
)

const (
	defaultContractSearchLimit    = 20
	defaultContractSearchPages      = 20
	defaultContractSearchPageSize   = 200
	maxContractSearchPagesHardCap   = 500
	stellarStrkeyAccountMinLen      = 56
)

// SearchContractsOptions configures contract discovery against Horizon /contracts.
type SearchContractsOptions struct {
	Query      string
	HorizonURL string
	Limit      int
	Timeout    time.Duration
	// MaxPages is the maximum number of Horizon pages to walk (each page is up to PageSize records).
	// Zero means default (defaultContractSearchPages).
	MaxPages int
	// PageSize is the Horizon "limit" per page (max 200). Zero means defaultContractSearchPageSize.
	PageSize int
}

// ContractSummary is a normalized contract row from Horizon contract list JSON.
type ContractSummary struct {
	ID                 string
	Symbol             string
	Creator            string
	LastModifiedLedger int64
	LastModifiedTime   string
}

// SearchContractsResult is returned by SearchContracts. It includes match rows plus
// scan metadata so callers can warn when results may be incomplete at scale.
type SearchContractsResult struct {
	Results []ContractSummary
	// IncompleteScan is true when pagination stopped due to MaxPages while Horizon still
	// advertised a next page — matches may exist beyond the scanned window.
	IncompleteScan bool
	// ScannedRecords is the number of contract rows examined across all pages (client-side filter input size).
	ScannedRecords int
	// MaxScanBudget is MaxPages * PageSize (upper bound on rows fetched from Horizon for this search).
	MaxScanBudget int
	// ServerSponsorFilter is true when the request used Horizon's sponsor= filter (account-shaped query).
	ServerSponsorFilter bool
}

type horizonContractsPage struct {
	Embedded struct {
		Records []map[string]any `json:"records"`
	} `json:"_embedded"`
	Links struct {
		Next struct {
			Href string `json:"href"`
		} `json:"next"`
	} `json:"_links"`
}

// SearchContracts walks Horizon /contracts pages, applies client-side matching for symbol / partial ID,
// and uses sponsor= when the query looks like a full Stellar account (strkey) for server-side filtering.
func SearchContracts(ctx context.Context, options SearchContractsOptions) (*SearchContractsResult, error) {
	query := strings.TrimSpace(options.Query)
	if query == "" {
		return nil, fmt.Errorf("query cannot be empty")
	}

	horizonURL := strings.TrimRight(strings.TrimSpace(options.HorizonURL), "/")
	if horizonURL == "" {
		horizonURL = strings.TrimRight(TestnetHorizonURL, "/")
	}

	limit := options.Limit
	if limit <= 0 {
		limit = defaultContractSearchLimit
	}

	maxPages := options.MaxPages
	if maxPages <= 0 {
		maxPages = defaultContractSearchPages
	}
	if maxPages > maxContractSearchPagesHardCap {
		maxPages = maxContractSearchPagesHardCap
	}

	pageSize := options.PageSize
	if pageSize <= 0 {
		pageSize = defaultContractSearchPageSize
	}
	if pageSize > 200 {
		pageSize = 200
	}

	timeout := options.Timeout
	if timeout <= 0 {
		timeout = 15 * time.Second
	}

	useSponsor := looksLikeStellarAccountStrkey(query)
	client := &http.Client{Timeout: timeout}
	nextURL := buildContractsURL(horizonURL, query, pageSize, useSponsor)
	results := make([]ContractSummary, 0, limit)
	scanned := 0
	var lastNext string

	for page := 0; page < maxPages && nextURL != ""; page++ {
		pageResults, next, nRecords, err := fetchContractPage(ctx, client, nextURL, query, limit-len(results))
		if err != nil {
			return nil, err
		}
		scanned += nRecords
		lastNext = next
		results = append(results, pageResults...)
		if len(results) >= limit {
			// Filled the user's --limit; if Horizon still has more pages, matches may exist beyond this window.
			hitLimitWithMore := strings.TrimSpace(next) != ""
			return &SearchContractsResult{
				Results:             results[:limit],
				IncompleteScan:      hitLimitWithMore,
				ScannedRecords:      scanned,
				MaxScanBudget:       maxPages * pageSize,
				ServerSponsorFilter: useSponsor,
			}, nil
		}
		nextURL = next
	}

	incomplete := strings.TrimSpace(lastNext) != ""
	return &SearchContractsResult{
		Results:             results,
		IncompleteScan:      incomplete,
		ScannedRecords:      scanned,
		MaxScanBudget:       maxPages * pageSize,
		ServerSponsorFilter: useSponsor,
	}, nil
}

func buildContractsURL(horizonURL, query string, pageSize int, sponsorFilter bool) string {
	u, _ := url.Parse(horizonURL)
	u.Path = path.Join(u.Path, "contracts")
	values := u.Query()
	values.Set("limit", strconv.Itoa(pageSize))
	values.Set("order", "desc")
	if sponsorFilter && looksLikeStellarAccountStrkey(query) {
		values.Set("sponsor", query)
	}
	u.RawQuery = values.Encode()
	return u.String()
}

func fetchContractPage(
	ctx context.Context,
	client *http.Client,
	pageURL string,
	query string,
	remaining int,
) ([]ContractSummary, string, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		return nil, "", 0, fmt.Errorf("build request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, "", 0, fmt.Errorf("fetch contracts: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, "", 0, fmt.Errorf("contracts endpoint returned HTTP %d", resp.StatusCode)
	}

	var page horizonContractsPage
	if err := json.NewDecoder(resp.Body).Decode(&page); err != nil {
		return nil, "", 0, fmt.Errorf("decode contracts response: %w", err)
	}

	nRecords := len(page.Embedded.Records)
	matches := make([]ContractSummary, 0, remaining)
	for _, raw := range page.Embedded.Records {
		summary := normalizeContractSummary(raw)
		if matchesQuery(summary, query) {
			matches = append(matches, summary)
			if len(matches) >= remaining {
				break
			}
		}
	}

	next := page.Links.Next.Href
	return matches, next, nRecords, nil
}

func normalizeContractSummary(raw map[string]any) ContractSummary {
	summary := ContractSummary{
		ID:                 firstNonEmpty(stringValue(raw["id"]), stringValue(raw["contract_id"])),
		Symbol:             firstNonEmpty(stringValue(raw["symbol"]), stringValue(raw["asset_code"])),
		Creator:            firstNonEmpty(stringValue(raw["creator"]), stringValue(raw["creator_account"]), stringValue(raw["sponsor"])),
		LastModifiedLedger: intValue(raw["last_modified_ledger"], raw["last_modified_ledger_seq"]),
		LastModifiedTime:   stringValue(raw["last_modified_time"]),
	}
	return summary
}

func matchesQuery(summary ContractSummary, query string) bool {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return false
	}
	fields := []string{summary.ID, summary.Symbol, summary.Creator}
	for _, field := range fields {
		if strings.Contains(strings.ToLower(field), q) {
			return true
		}
	}
	return false
}

// looksLikeStellarAccountStrkey returns true for a classic account strkey (G…, 56 chars).
func looksLikeStellarAccountStrkey(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) != stellarStrkeyAccountMinLen {
		return false
	}
	if s[0] != 'G' {
		return false
	}
	for i := 1; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' || c >= '2' && c <= '7' {
			continue
		}
		return false
	}
	return true
}

func stringValue(v any) string {
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(s)
}

func intValue(values ...any) int64 {
	for _, v := range values {
		switch n := v.(type) {
		case float64:
			return int64(n)
		case int64:
			return n
		case int:
			return int64(n)
		case json.Number:
			if parsed, err := n.Int64(); err == nil {
				return parsed
			}
		}
	}
	return 0
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
