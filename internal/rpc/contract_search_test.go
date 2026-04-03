// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSearchContracts_MatchesBySymbolCreatorAndPartialID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/contracts" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"_embedded": map[string]any{
				"records": []map[string]any{
					{
						"id":                   "CAAA111",
						"symbol":               "USDC",
						"creator":              "GCREATORA",
						"last_modified_ledger": 123,
						"last_modified_time":   "2026-03-20T01:02:03Z",
					},
					{
						"id":                   "CBBB222",
						"symbol":               "MEME",
						"creator":              "GCREATORB",
						"last_modified_ledger": 456,
						"last_modified_time":   "2026-03-21T01:02:03Z",
					},
				},
			},
			"_links": map[string]any{
				"next": map[string]any{"href": ""},
			},
		})
	}))
	defer server.Close()

	ctx := context.Background()

	bySymbol, err := SearchContracts(ctx, SearchContractsOptions{
		Query:      "usdc",
		HorizonURL: server.URL,
		Limit:      10,
	})
	if err != nil {
		t.Fatalf("search by symbol failed: %v", err)
	}
	if len(bySymbol.Results) != 1 || bySymbol.Results[0].ID != "CAAA111" {
		t.Fatalf("unexpected symbol results: %+v", bySymbol.Results)
	}

	byCreator, err := SearchContracts(ctx, SearchContractsOptions{
		Query:      "GCREATORB",
		HorizonURL: server.URL,
		Limit:      10,
	})
	if err != nil {
		t.Fatalf("search by creator failed: %v", err)
	}
	if len(byCreator.Results) != 1 || byCreator.Results[0].ID != "CBBB222" {
		t.Fatalf("unexpected creator results: %+v", byCreator.Results)
	}

	byPartialID, err := SearchContracts(ctx, SearchContractsOptions{
		Query:      "caaa",
		HorizonURL: server.URL,
		Limit:      10,
	})
	if err != nil {
		t.Fatalf("search by id failed: %v", err)
	}
	if len(byPartialID.Results) != 1 || byPartialID.Results[0].ID != "CAAA111" {
		t.Fatalf("unexpected partial id results: %+v", byPartialID.Results)
	}
}

func TestSearchContracts_RespectsLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"_embedded": map[string]any{
				"records": []map[string]any{
					{"id": "CA1", "symbol": "ABC", "creator": "G1", "last_modified_ledger": 10},
					{"id": "CA2", "symbol": "ABC", "creator": "G2", "last_modified_ledger": 20},
					{"id": "CA3", "symbol": "ABC", "creator": "G3", "last_modified_ledger": 30},
				},
			},
			"_links": map[string]any{
				"next": map[string]any{"href": ""},
			},
		})
	}))
	defer server.Close()

	results, err := SearchContracts(context.Background(), SearchContractsOptions{
		Query:      "abc",
		HorizonURL: server.URL,
		Limit:      2,
	})
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if len(results.Results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results.Results))
	}
}

func TestSearchContracts_IncompleteScanWhenNextLinkPresent(t *testing.T) {
	nextURL := ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"_embedded": map[string]any{
				"records": []map[string]any{
					{"id": "CX1", "symbol": "NOPE", "creator": "G1"},
				},
			},
			"_links": map[string]any{
				"next": map[string]any{"href": nextURL},
			},
		})
	}))
	defer server.Close()
	nextURL = server.URL + "/contracts?cursor=abc"

	res, err := SearchContracts(context.Background(), SearchContractsOptions{
		Query:      "missing",
		HorizonURL: server.URL,
		Limit:      5,
		MaxPages:   1,
	})
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if !res.IncompleteScan {
		t.Fatal("expected IncompleteScan when next link is set and max pages reached")
	}
	if res.ScannedRecords != 1 {
		t.Fatalf("ScannedRecords = %d, want 1", res.ScannedRecords)
	}
}

func TestSearchContracts_SponsorServerFilterForAccountStrkey(t *testing.T) {
	account := "G" + strings.Repeat("A", 55)
	var sawSponsor string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawSponsor = r.URL.Query().Get("sponsor")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"_embedded": map[string]any{
				"records": []map[string]any{
					{"id": "CC1", "symbol": "X", "creator": account, "sponsor": account},
				},
			},
			"_links": map[string]any{
				"next": map[string]any{"href": ""},
			},
		})
	}))
	defer server.Close()

	res, err := SearchContracts(context.Background(), SearchContractsOptions{
		Query:      account,
		HorizonURL: server.URL,
		Limit:      10,
	})
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if sawSponsor != account {
		t.Fatalf("sponsor query param = %q, want %q", sawSponsor, account)
	}
	if !res.ServerSponsorFilter {
		t.Fatal("expected ServerSponsorFilter true")
	}
	if len(res.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(res.Results))
	}
}

func TestLooksLikeStellarAccountStrkey(t *testing.T) {
	acct := "G" + strings.Repeat("A", 55)
	if !looksLikeStellarAccountStrkey(acct) {
		t.Fatal("expected valid account strkey shape")
	}
	contract := "C" + strings.Repeat("A", 55)
	if looksLikeStellarAccountStrkey(contract) {
		t.Fatal("contract strkey must not match account check")
	}
	if looksLikeStellarAccountStrkey("GSHORT") {
		t.Fatal("short string must not match")
	}
}
