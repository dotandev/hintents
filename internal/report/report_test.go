// Copyright 2026 dotandev
// SPDX-License-Identifier: Apache-2.0

package report

import (
	"strings"
	"testing"
	"time"
)

func TestReportCreation(t *testing.T) {
	report := NewReport("Test Transaction")

	if report.Title != "Test Transaction" {
		t.Errorf("expected title 'Test Transaction', got %q", report.Title)
	}

	if report.GeneratedAt.IsZero() {
		t.Error("expected non-zero GeneratedAt")
	}
}

func TestBuilderChaining(t *testing.T) {
	report := NewBuilder("Test Report").
		WithTransactionHash("0xdef456").
		SetSummary("success", "100ms", 5, 2, 0, 95.5).
		AddKeyFinding("Critical issue detected").
		AddKeyFinding("Performance bottleneck").
		Build()

	if report.Execution.TransactionHash != "0xdef456" {
		t.Errorf("expected tx hash, got %q", report.Execution.TransactionHash)
	}

	if report.Summary == nil {
		t.Fatal("expected summary to be set")
	}

	if len(report.Summary.KeyFindings) != 2 {
		t.Errorf("expected 2 findings, got %d", len(report.Summary.KeyFindings))
	}
}

func TestExecutionStepTracking(t *testing.T) {
	builder := NewBuilder("Test Report")
	builder.AddExecutionStep(0, "decode", "success", "Decoded transaction")
	builder.AddExecutionStep(1, "validate", "success", "Validation passed")
	report := builder.Build()

	if report.Execution == nil {
		t.Fatal("expected execution log to be set")
	}

	if len(report.Execution.Steps) != 2 {
		t.Errorf("expected 2 execution steps, got %d", len(report.Execution.Steps))
	}

	if report.Execution.Steps[0].Operation != "decode" {
		t.Errorf("expected first step 'decode', got %q", report.Execution.Steps[0].Operation)
	}
}

func TestContractMetrics(t *testing.T) {
	metric := &ContractMetric{
		CallCount:   50,
		ErrorCount:  3,
		AvgDuration: "25ms",
		Functions:   []string{"transfer", "approve", "balanceOf"},
	}

	if metric.CallCount != 50 {
		t.Errorf("expected 50 calls, got %d", metric.CallCount)
	}

	if len(metric.Functions) != 3 {
		t.Errorf("expected 3 functions, got %d", len(metric.Functions))
	}
}

func TestRiskAssessment(t *testing.T) {
	builder := NewBuilder("Test Report")
	builder.SetRiskAssessment("medium", 65)
	report := builder.Build()

	if report.Analytics == nil || report.Analytics.RiskAssessment == nil {
		t.Fatal("expected risk assessment to be set")
	}

	if report.Analytics.RiskAssessment.Level != "medium" {
		t.Errorf("expected risk level 'medium', got %q", report.Analytics.RiskAssessment.Level)
	}

	if report.Analytics.RiskAssessment.Score != 65 {
		t.Errorf("expected risk score 65, got %f", report.Analytics.RiskAssessment.Score)
	}
}

func TestHTMLRendering(t *testing.T) {
	report := NewBuilder("Test Report").
		WithTransactionHash("0xtest").
		SetSummary("success", "50ms", 10, 1, 0, 99.0).
		AddKeyFinding("Test finding").
		Build()

	renderer := NewHTMLRenderer()
	html, err := renderer.Render(report)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(html) == 0 {
		t.Error("expected non-empty HTML output")
	}

	htmlStr := string(html)
	if !strings.Contains(htmlStr, "<!DOCTYPE html>") {
		t.Error("expected valid HTML document")
	}

	if !strings.Contains(htmlStr, "0xtest") {
		t.Error("expected transaction hash in HTML")
	}

	if !strings.Contains(htmlStr, "Test finding") {
		t.Error("expected key finding in HTML")
	}
}

func TestPDFGeneration(t *testing.T) {
	report := NewBuilder("Test Report").
		WithTransactionHash("0xpdf").
		SetSummary("success", "75ms", 8, 0, 1, 98.5).
		Build()

	renderer := NewPDFRenderer()
	pdf, err := renderer.Render(report)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(pdf) == 0 {
		t.Error("expected non-empty PDF output")
	}
}

func TestJSONExport(t *testing.T) {
	builder := NewBuilder("Test Report").
		WithTransactionHash("0xjson").
		SetSummary("success", "60ms", 12, 2, 0, 97.5).
		AddKeyFinding("JSON test")

	json, err := builder.ExportJSON()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(json) == 0 {
		t.Error("expected non-empty JSON output")
	}

	jsonStr := string(json)
	if !strings.Contains(jsonStr, "0xjson") {
		t.Error("expected transaction hash in JSON")
	}
}

func TestEventDistribution(t *testing.T) {
	builder := NewBuilder("Test Report")
	builder.RecordEvent("transaction", 5)
	builder.RecordEvent("call", 10)
	builder.RecordEvent("transaction", 3)
	report := builder.Build()

	if report.Analytics.EventDistribution["transaction"] != 8 {
		t.Errorf("expected 8 transactions, got %d", report.Analytics.EventDistribution["transaction"])
	}

	if report.Analytics.EventDistribution["call"] != 10 {
		t.Errorf("expected 10 calls, got %d", report.Analytics.EventDistribution["call"])
	}
}

func TestMetadata(t *testing.T) {
	meta := &Metadata{
		GeneratorVersion: "1.0.0",
		DataSource:       "production",
		ExportTime:       time.Now(),
	}

	if meta.GeneratorVersion != "1.0.0" {
		t.Errorf("expected version '1.0.0', got %q", meta.GeneratorVersion)
	}

	if meta.DataSource != "production" {
		t.Errorf("expected data source 'production', got %q", meta.DataSource)
	}

	if meta.ExportTime.IsZero() {
		t.Error("expected non-zero export time")
	}
}

func TestHTMLEscaping(t *testing.T) {
	report := NewBuilder("Test Report").
		AddKeyFinding("<script>alert('xss')</script>").
		Build()

	renderer := NewHTMLRenderer()
	html, err := renderer.Render(report)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	htmlStr := string(html)
	if strings.Contains(htmlStr, "<script>") {
		t.Error("script tag should be escaped in HTML output")
	}
}

func TestEmptyReport(t *testing.T) {
	report := NewReport("Empty")

	if report.Title != "Empty" {
		t.Errorf("expected title 'Empty', got %q", report.Title)
	}

	if report.Summary == nil {
		t.Error("expected summary to be initialized")
	}

	if report.Execution == nil {
		t.Error("expected execution to be initialized")
	}
}

func TestReportTimestamp(t *testing.T) {
	before := time.Now()
	report := NewReport("Test")
	after := time.Now()

	if report.GeneratedAt.Before(before) || report.GeneratedAt.After(after) {
		t.Errorf("report timestamp outside expected range: %v", report.GeneratedAt)
	}
}

func TestBuilderIssues(t *testing.T) {
	builder := NewBuilder("Test Report")
	builder.AddIssue("security", "high", "SQL injection vulnerability", "0xtoken", "transfer()")
	builder.AddIssue("gas", "medium", "Inefficient loop", "0xtoken", "updateBalances()")
	report := builder.Build()

	if len(report.Analytics.RiskAssessment.Issues) != 2 {
		t.Errorf("expected 2 issues, got %d", len(report.Analytics.RiskAssessment.Issues))
	}

	if report.Analytics.RiskAssessment.Issues[0].Severity != "high" {
		t.Errorf("expected first issue high severity, got %q", report.Analytics.RiskAssessment.Issues[0].Severity)
	}
}

func TestBuilderWarnings(t *testing.T) {
	builder := NewBuilder("Test Report")
	builder.AddWarning("Deprecated function used")
	builder.AddWarning("Performance issue detected")
	report := builder.Build()

	if len(report.Analytics.RiskAssessment.Warnings) != 2 {
		t.Errorf("expected 2 warnings, got %d", len(report.Analytics.RiskAssessment.Warnings))
	}
}

func TestBuilderFromJSON(t *testing.T) {
	original := NewBuilder("Original").
		WithTransactionHash("0xoriginal").
		SetSummary("success", "50ms", 5, 0, 1, 100.0)

	jsonData, _ := original.ExportJSON()

	builder := NewBuilder("temp")
	builder, err := builder.FromJSON(jsonData)
	if err != nil {
		t.Fatalf("failed to load from JSON: %v", err)
	}

	report := builder.Build()
	if report.Title != "Original" {
		t.Errorf("expected title 'Original', got %q", report.Title)
	}
}

func BenchmarkHTMLRendering(b *testing.B) {
	report := NewBuilder("Test Report").
		WithTransactionHash("0xbench").
		SetSummary("success", "100ms", 20, 3, 1, 96.5).
		AddKeyFinding("Benchmark test").
		Build()

	renderer := NewHTMLRenderer()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = renderer.Render(report)
	}
}

func BenchmarkBuilderOperations(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewBuilder("Report").
			WithTransactionHash("0x123").
			SetSummary("success", "50ms", 10, 0, 0, 100.0).
			AddKeyFinding("Test").
			Build()
	}
}
