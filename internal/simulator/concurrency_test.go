// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package simulator

import (
	"encoding/json"
	"sync"
	"testing"
)

func TestConcurrentReadOnlyMockRunnerAccess(t *testing.T) {
	mock := NewDefaultMockRunner()

	req := &SimulationRequest{
		EnvelopeXdr:   "AAAA_envelope",
		ResultMetaXdr: "AAAA_meta",
		LedgerEntries: map[string]string{
			"contract_a": "state_value_a",
			"contract_b": "state_value_b",
			"contract_c": "state_value_c",
		},
		LedgerSequence: 50000,
		Timestamp:      1700000000,
	}

	const workers = 16
	var wg sync.WaitGroup
	wg.Add(workers)

	errs := make(chan error, workers)

	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			resp, err := mock.Run(req)
			if err != nil {
				errs <- err
				return
			}
			if resp.Status != "success" {
				errs <- err
			}
		}()
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Errorf("concurrent Run returned error: %v", err)
	}
}

func TestConcurrentLedgerStateReads(t *testing.T) {
	ledger := map[string]string{
		"key_alpha":   "val_alpha",
		"key_beta":    "val_beta",
		"key_gamma":   "val_gamma",
		"key_delta":   "val_delta",
		"key_epsilon": "val_epsilon",
	}

	req := &SimulationRequest{
		EnvelopeXdr:    "envelope_xdr",
		ResultMetaXdr:  "result_meta_xdr",
		LedgerEntries:  ledger,
		LedgerSequence: 12345,
	}

	const readers = 32
	var wg sync.WaitGroup
	wg.Add(readers)

	for i := 0; i < readers; i++ {
		go func() {
			defer wg.Done()
			for k, v := range req.LedgerEntries {
				if k == "" || v == "" {
					t.Errorf("unexpected empty key or value")
				}
			}
			if req.EnvelopeXdr != "envelope_xdr" {
				t.Errorf("envelope changed during concurrent read")
			}
			if req.LedgerSequence != 12345 {
				t.Errorf("ledger sequence changed during concurrent read")
			}
		}()
	}

	wg.Wait()
}

func TestConcurrentResponseDeserialization(t *testing.T) {
	respJSON := `{
		"status": "success",
		"events": ["ev1", "ev2"],
		"logs": ["log1"],
		"budget_usage": {
			"cpu_instructions": 80000000,
			"memory_bytes": 30000000,
			"operations_count": 15,
			"cpu_limit": 100000000,
			"memory_limit": 50000000,
			"cpu_usage_percent": 80.0,
			"memory_usage_percent": 60.0
		}
	}`

	const workers = 20
	var wg sync.WaitGroup
	wg.Add(workers)

	errs := make(chan error, workers)

	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			var resp SimulationResponse
			if err := json.Unmarshal([]byte(respJSON), &resp); err != nil {
				errs <- err
				return
			}
			if resp.Status != "success" {
				t.Errorf("expected success, got %s", resp.Status)
			}
			if resp.BudgetUsage == nil {
				t.Errorf("expected non-nil BudgetUsage")
			}
		}()
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Errorf("concurrent unmarshal error: %v", err)
	}
}

func TestConcurrentMockRunnerWithSharedLedger(t *testing.T) {
	ledger := map[string]string{
		"contract_1": "data_1",
		"contract_2": "data_2",
		"contract_3": "data_3",
	}

	mock := NewMockRunner(func(req *SimulationRequest) (*SimulationResponse, error) {
		events := make([]string, 0, len(req.LedgerEntries))
		for k := range req.LedgerEntries {
			events = append(events, "read:"+k)
		}
		return &SimulationResponse{
			Status: "success",
			Events: events,
		}, nil
	})

	const workers = 24
	var wg sync.WaitGroup
	wg.Add(workers)

	errs := make(chan error, workers)

	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			req := &SimulationRequest{
				EnvelopeXdr:   "envelope",
				ResultMetaXdr: "meta",
				LedgerEntries: ledger,
			}
			resp, err := mock.Run(req)
			if err != nil {
				errs <- err
				return
			}
			if resp.Status != "success" {
				t.Errorf("expected success, got %s", resp.Status)
			}
			if len(resp.Events) != len(ledger) {
				t.Errorf("expected %d events, got %d", len(ledger), len(resp.Events))
			}
		}()
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Errorf("concurrent run error: %v", err)
	}
}

func TestConcurrentBuilderReadOnlyAccess(t *testing.T) {
	req, err := NewSimulationRequestBuilder().
		WithEnvelopeXDR("envelope_xdr_data").
		WithResultMetaXDR("result_meta_xdr_data").
		WithLedgerEntry("key1", "value1").
		WithLedgerEntry("key2", "value2").
		WithLedgerEntry("key3", "value3").
		Build()
	if err != nil {
		t.Fatalf("builder error: %v", err)
	}

	mock := NewDefaultMockRunner()

	const workers = 16
	var wg sync.WaitGroup
	wg.Add(workers)

	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			resp, err := mock.Run(req)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if resp.Status != "success" {
				t.Errorf("expected success, got %s", resp.Status)
			}
			if req.EnvelopeXdr != "envelope_xdr_data" {
				t.Errorf("request mutated during concurrent access")
			}
			if len(req.LedgerEntries) != 3 {
				t.Errorf("ledger entries count changed: got %d", len(req.LedgerEntries))
			}
		}()
	}

	wg.Wait()
}

func TestConcurrentProtocolReadAccess(t *testing.T) {
	const readers = 20
	var wg sync.WaitGroup
	wg.Add(readers)

	for i := 0; i < readers; i++ {
		go func() {
			defer wg.Done()
			p := GetOrDefault(nil)
			if p.Version != LatestVersion() {
				t.Errorf("expected version %d, got %d", LatestVersion(), p.Version)
			}
			v := uint32(20)
			p20 := GetOrDefault(&v)
			if p20.Version != 20 {
				t.Errorf("expected version 20, got %d", p20.Version)
			}
			supported := Supported()
			if len(supported) < 3 {
				t.Errorf("expected at least 3 protocols, got %d", len(supported))
			}
		}()
	}

	wg.Wait()
}
