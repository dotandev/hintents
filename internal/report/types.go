// Copyright 2026 dotandev
// SPDX-License-Identifier: Apache-2.0

package report

import "time"

type Format string

const (
	FormatHTML Format = "html"
	FormatPDF  Format = "pdf"
)

type Report struct {
	Title       string        `json:"title"`
	GeneratedAt time.Time     `json:"generated_at"`
	Summary     *Summary      `json:"summary"`
	Execution   *ExecutionLog `json:"execution"`
	Analytics   *Analytics    `json:"analytics"`
	Metadata    *Metadata     `json:"metadata"`
}

type Summary struct {
	Status          string   `json:"status"`
	Duration        string   `json:"duration"`
	TotalEvents     int      `json:"total_events"`
	TotalErrors     int      `json:"total_errors"`
	ContractsCalled int      `json:"contracts_called"`
	SuccessRate     float64  `json:"success_rate"`
	KeyFindings     []string `json:"key_findings"`
}

type ExecutionLog struct {
	TransactionHash string          `json:"transaction_hash"`
	Steps           []ExecutionStep `json:"steps"`
	ErrorTrace      []string        `json:"error_trace,omitempty"`
	CallStack       []CallInfo      `json:"call_stack,omitempty"`
}

type ExecutionStep struct {
	Index      int                    `json:"index"`
	Timestamp  int64                  `json:"timestamp"`
	Operation  string                 `json:"operation"`
	ContractID string                 `json:"contract_id,omitempty"`
	Function   string                 `json:"function,omitempty"`
	Status     string                 `json:"status"`
	Details    string                 `json:"details,omitempty"`
	Input      map[string]interface{} `json:"input,omitempty"`
	Output     map[string]interface{} `json:"output,omitempty"`
}

type CallInfo struct {
	Depth      int    `json:"depth"`
	ContractID string `json:"contract_id"`
	Function   string `json:"function"`
	Status     string `json:"status"`
}

type Analytics struct {
	EventDistribution map[string]int             `json:"event_distribution"`
	ContractMetrics   map[string]*ContractMetric `json:"contract_metrics"`
	TimelineData      []TimelinePoint            `json:"timeline_data"`
	RiskAssessment    *RiskAssessment            `json:"risk_assessment"`
}

type ContractMetric struct {
	CallCount   int      `json:"call_count"`
	ErrorCount  int      `json:"error_count"`
	AvgDuration string   `json:"avg_duration"`
	Functions   []string `json:"functions"`
}

type TimelinePoint struct {
	Timestamp int64  `json:"timestamp"`
	EventType string `json:"event_type"`
	Count     int    `json:"count"`
}

type RiskAssessment struct {
	Level    string   `json:"level"`
	Score    float64  `json:"score"`
	Issues   []Issue  `json:"issues"`
	Warnings []string `json:"warnings"`
}

type Issue struct {
	Type        string `json:"type"`
	Severity    string `json:"severity"`
	Description string `json:"description"`
	Contract    string `json:"contract,omitempty"`
	Location    string `json:"location,omitempty"`
}

type Metadata struct {
	GeneratorVersion string            `json:"generator_version"`
	DataSource       string            `json:"data_source"`
	ExportTime       time.Time         `json:"export_time"`
	Tags             map[string]string `json:"tags,omitempty"`
}

func NewReport(title string) *Report {
	return &Report{
		Title:       title,
		GeneratedAt: time.Now(),
		Summary:     &Summary{},
		Execution:   &ExecutionLog{},
		Analytics: &Analytics{
			EventDistribution: make(map[string]int),
			ContractMetrics:   make(map[string]*ContractMetric),
			TimelineData:      make([]TimelinePoint, 0),
		},
		Metadata: &Metadata{
			GeneratorVersion: "1.0.0",
			ExportTime:       time.Now(),
		},
	}
}
