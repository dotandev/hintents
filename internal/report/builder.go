// Copyright 2026 dotandev
// SPDX-License-Identifier: Apache-2.0

package report

import (
	"encoding/json"
	"fmt"
	"time"
)

type Builder struct {
	report *Report
}

func NewBuilder(title string) *Builder {
	return &Builder{
		report: NewReport(title),
	}
}

func (b *Builder) WithTransactionHash(hash string) *Builder {
	if b.report.Execution == nil {
		b.report.Execution = &ExecutionLog{}
	}
	b.report.Execution.TransactionHash = hash
	return b
}

func (b *Builder) AddExecutionStep(index int, op string, status string, details string) *Builder {
	if b.report.Execution == nil {
		b.report.Execution = &ExecutionLog{}
	}

	step := ExecutionStep{
		Index:     index,
		Timestamp: time.Now().UnixMilli(),
		Operation: op,
		Status:    status,
		Details:   details,
	}

	b.report.Execution.Steps = append(b.report.Execution.Steps, step)
	return b
}

func (b *Builder) AddContractCall(contractID, function, status string) *Builder {
	if b.report.Execution == nil {
		b.report.Execution = &ExecutionLog{}
	}

	step := ExecutionStep{
		Index:      len(b.report.Execution.Steps),
		Timestamp:  time.Now().UnixMilli(),
		Operation:  "contract_call",
		ContractID: contractID,
		Function:   function,
		Status:     status,
	}

	b.report.Execution.Steps = append(b.report.Execution.Steps, step)
	return b
}

func (b *Builder) AddContractMetric(contractID string, metric *ContractMetric) *Builder {
	if b.report.Analytics.ContractMetrics == nil {
		b.report.Analytics.ContractMetrics = make(map[string]*ContractMetric)
	}
	b.report.Analytics.ContractMetrics[contractID] = metric
	return b
}

func (b *Builder) RecordEvent(eventType string, count int) *Builder {
	if b.report.Analytics.EventDistribution == nil {
		b.report.Analytics.EventDistribution = make(map[string]int)
	}
	b.report.Analytics.EventDistribution[eventType] += count
	return b
}

func (b *Builder) SetSummary(status string, duration string, totalEvents, totalErrors, contracts int, successRate float64) *Builder {
	b.report.Summary.Status = status
	b.report.Summary.Duration = duration
	b.report.Summary.TotalEvents = totalEvents
	b.report.Summary.TotalErrors = totalErrors
	b.report.Summary.ContractsCalled = contracts
	b.report.Summary.SuccessRate = successRate
	return b
}

func (b *Builder) AddKeyFinding(finding string) *Builder {
	b.report.Summary.KeyFindings = append(b.report.Summary.KeyFindings, finding)
	return b
}

func (b *Builder) AddIssue(issueType, severity, description, contract, location string) *Builder {
	if b.report.Analytics.RiskAssessment == nil {
		b.report.Analytics.RiskAssessment = &RiskAssessment{
			Issues:   make([]Issue, 0),
			Warnings: make([]string, 0),
		}
	}

	issue := Issue{
		Type:        issueType,
		Severity:    severity,
		Description: description,
		Contract:    contract,
		Location:    location,
	}

	b.report.Analytics.RiskAssessment.Issues = append(b.report.Analytics.RiskAssessment.Issues, issue)
	return b
}

func (b *Builder) AddWarning(warning string) *Builder {
	if b.report.Analytics.RiskAssessment == nil {
		b.report.Analytics.RiskAssessment = &RiskAssessment{
			Issues:   make([]Issue, 0),
			Warnings: make([]string, 0),
		}
	}
	b.report.Analytics.RiskAssessment.Warnings = append(b.report.Analytics.RiskAssessment.Warnings, warning)
	return b
}

func (b *Builder) SetRiskAssessment(level string, score float64) *Builder {
	if b.report.Analytics.RiskAssessment == nil {
		b.report.Analytics.RiskAssessment = &RiskAssessment{
			Issues:   make([]Issue, 0),
			Warnings: make([]string, 0),
		}
	}
	b.report.Analytics.RiskAssessment.Level = level
	b.report.Analytics.RiskAssessment.Score = score
	return b
}

func (b *Builder) SetMetadata(source, version string, tags map[string]string) *Builder {
	b.report.Metadata.DataSource = source
	b.report.Metadata.GeneratorVersion = version
	if tags != nil {
		b.report.Metadata.Tags = tags
	}
	return b
}

func (b *Builder) FromJSON(data []byte) (*Builder, error) {
	var report Report
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}
	b.report = &report
	return b, nil
}

func (b *Builder) Build() *Report {
	if b.report.Summary == nil {
		b.report.Summary = &Summary{}
	}
	if b.report.Execution == nil {
		b.report.Execution = &ExecutionLog{}
	}
	if b.report.Analytics == nil {
		b.report.Analytics = &Analytics{
			EventDistribution: make(map[string]int),
			ContractMetrics:   make(map[string]*ContractMetric),
		}
	}
	if b.report.Metadata == nil {
		b.report.Metadata = &Metadata{}
	}
	return b.report
}

func (b *Builder) ExportJSON() ([]byte, error) {
	return json.MarshalIndent(b.Build(), "", "  ")
}

func (b *Builder) ExportHTML() ([]byte, error) {
	renderer := NewHTMLRenderer()
	return renderer.Render(b.Build())
}

func (b *Builder) ExportPDF() ([]byte, error) {
	renderer := NewPDFRenderer()
	return renderer.Render(b.Build())
}
