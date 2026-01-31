// Copyright 2026 dotandev
// SPDX-License-Identifier: Apache-2.0

package report

import (
	"bytes"
	"fmt"
	"html"
	"strings"
	"text/template"
	"time"
)

type HTMLRenderer struct {
	tmpl *template.Template
}

func NewHTMLRenderer() *HTMLRenderer {
	return &HTMLRenderer{}
}

func (r *HTMLRenderer) Render(report *Report) ([]byte, error) {
	tmpl, err := template.New("report").Funcs(template.FuncMap{
		"formatTime":  formatTime,
		"escapeHTML":  escapeHTML,
		"statusClass": statusClass,
		"riskColor":   riskColor,
	}).Parse(htmlTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, report); err != nil {
		return nil, fmt.Errorf("failed to render template: %w", err)
	}

	return buf.Bytes(), nil
}

func formatTime(t time.Time) string {
	return t.Format("2006-01-02 15:04:05")
}

func escapeHTML(s string) string {
	return html.EscapeString(s)
}

func statusClass(status string) string {
	switch status {
	case "success":
		return "status-success"
	case "error":
		return "status-error"
	case "warning":
		return "status-warning"
	default:
		return "status-unknown"
	}
}

func riskColor(level string) string {
	switch strings.ToLower(level) {
	case "critical":
		return "#d32f2f"
	case "high":
		return "#f57c00"
	case "medium":
		return "#fbc02d"
	case "low":
		return "#388e3c"
	default:
		return "#9e9e9e"
	}
}

const htmlTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
	<meta charset="UTF-8">
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
	<title>{{ .Title }}</title>
	<style>
		* { margin: 0; padding: 0; box-sizing: border-box; }
		body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; color: #333; background: #f5f5f5; line-height: 1.6; }
		.container { max-width: 1200px; margin: 0 auto; background: white; box-shadow: 0 0 10px rgba(0,0,0,0.1); }
		header { background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); color: white; padding: 40px 30px; text-align: center; }
		header h1 { font-size: 2.5em; margin-bottom: 10px; }
		.header-meta { font-size: 0.9em; opacity: 0.9; }
		.toc { background: #f9f9f9; border-bottom: 1px solid #e0e0e0; padding: 20px 30px; display: flex; gap: 30px; flex-wrap: wrap; }
		.toc a { color: #667eea; text-decoration: none; font-weight: 500; }
		.toc a:hover { color: #764ba2; }
		section { padding: 40px 30px; border-bottom: 1px solid #e0e0e0; }
		section:last-child { border-bottom: none; }
		h2 { color: #667eea; font-size: 1.8em; margin-bottom: 20px; padding-bottom: 10px; border-bottom: 2px solid #667eea; }
		h3 { color: #764ba2; font-size: 1.2em; margin: 20px 0 10px 0; }
		.summary-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 20px; margin-bottom: 30px; }
		.summary-card { background: #f5f5f5; border-left: 4px solid #667eea; padding: 20px; border-radius: 4px; }
		.summary-card h4 { color: #666; font-size: 0.9em; text-transform: uppercase; margin-bottom: 10px; }
		.summary-card .value { font-size: 2em; font-weight: bold; color: #333; }
		.status-success { color: #388e3c; }
		.status-error { color: #d32f2f; }
		.status-warning { color: #f57c00; }
		.status-unknown { color: #9e9e9e; }
		table { width: 100%; border-collapse: collapse; margin: 20px 0; }
		thead { background: #f5f5f5; font-weight: 600; }
		th, td { padding: 12px 15px; text-align: left; border-bottom: 1px solid #e0e0e0; }
		tbody tr:hover { background: #fafafa; }
		.alert { padding: 15px 20px; border-radius: 4px; margin: 15px 0; }
		.alert-warning { background: #fff3e0; border-left: 4px solid #fbc02d; color: #e65100; }
		.alert-danger { background: #ffebee; border-left: 4px solid #d32f2f; color: #c62828; }
		.risk-score { font-size: 3em; font-weight: bold; text-align: center; padding: 20px; border-radius: 8px; }
		.metric-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(250px, 1fr)); gap: 20px; margin: 20px 0; }
		.metric-box { background: #f9f9f9; padding: 20px; border-radius: 4px; border: 1px solid #e0e0e0; }
		.metric-box h4 { color: #667eea; margin-bottom: 10px; }
		.metric-box .metric-value { font-size: 1.5em; font-weight: bold; color: #333; }
		footer { background: #f5f5f5; padding: 20px 30px; text-align: center; color: #999; font-size: 0.9em; }
		@media print { body { background: white; } .container { box-shadow: none; } section { page-break-inside: avoid; } }
	</style>
</head>
<body>
	<div class="container">
		<header>
			<h1>{{ .Title }}</h1>
			<div class="header-meta">Generated on {{ formatTime .GeneratedAt }}</div>
		</header>
		<div class="toc">
			<a href="#summary">Summary</a>
			<a href="#execution">Execution</a>
			<a href="#analytics">Analytics</a>
			<a href="#risks">Risk Assessment</a>
			<a href="#metadata">Metadata</a>
		</div>
		<section id="summary">
			<h2>Executive Summary</h2>
			{{ with .Summary }}
			<div class="summary-grid">
				<div class="summary-card">
					<h4>Status</h4>
					<div class="value {{ statusClass .Status }}">{{ .Status }}</div>
				</div>
				<div class="summary-card">
					<h4>Total Events</h4>
					<div class="value">{{ .TotalEvents }}</div>
				</div>
				<div class="summary-card">
					<h4>Errors</h4>
					<div class="value">{{ .TotalErrors }}</div>
				</div>
				<div class="summary-card">
					<h4>Success Rate</h4>
					<div class="value">{{ printf "%.1f" .SuccessRate }}%</div>
				</div>
			</div>
			{{ if .KeyFindings }}
			<h3>Key Findings</h3>
			<ul>
				{{ range .KeyFindings }}<li>{{ escapeHTML . }}</li>{{ end }}
			</ul>
			{{ end }}
			{{ end }}
		</section>
		<section id="execution">
			<h2>Execution Details</h2>
			{{ with .Execution }}
			{{ if .TransactionHash }}<p><strong>Transaction Hash:</strong> <code>{{ .TransactionHash }}</code></p>{{ end }}
			{{ if .Steps }}
			<h3>Execution Steps</h3>
			<table>
				<thead>
					<tr><th>#</th><th>Operation</th><th>Contract/Function</th><th>Status</th><th>Details</th></tr>
				</thead>
				<tbody>
					{{ range .Steps }}
					<tr>
						<td>{{ .Index }}</td>
						<td>{{ .Operation }}</td>
						<td>{{ if .ContractID }}{{ .ContractID }}::{{ .Function }}{{ else }}{{ .Function }}{{ end }}</td>
						<td><span class="{{ statusClass .Status }}">{{ .Status }}</span></td>
						<td>{{ .Details }}</td>
					</tr>
					{{ end }}
				</tbody>
			</table>
			{{ end }}
			{{ if .ErrorTrace }}
			<h3>Error Trace</h3>
			<div class="alert alert-danger">{{ range .ErrorTrace }}<div>{{ escapeHTML . }}</div>{{ end }}</div>
			{{ end }}
			{{ end }}
		</section>
		<section id="analytics">
			<h2>Analytics & Metrics</h2>
			{{ with .Analytics }}
			{{ if .ContractMetrics }}
			<h3>Contract Statistics</h3>
			<div class="metric-grid">
				{{ range $contract, $metric := .ContractMetrics }}
				<div class="metric-box">
					<h4>{{ $contract }}</h4>
					<div class="metric-value">{{ $metric.CallCount }} calls</div>
					<p>Errors: {{ $metric.ErrorCount }}</p>
					<p>Avg: {{ $metric.AvgDuration }}</p>
				</div>
				{{ end }}
			</div>
			{{ end }}
			{{ if .EventDistribution }}
			<h3>Event Distribution</h3>
			<table>
				<thead><tr><th>Event Type</th><th>Count</th></tr></thead>
				<tbody>
					{{ range $type, $count := .EventDistribution }}
					<tr><td>{{ $type }}</td><td>{{ $count }}</td></tr>
					{{ end }}
				</tbody>
			</table>
			{{ end }}
			{{ end }}
		</section>
		<section id="risks">
			<h2>Risk Assessment</h2>
			{{ with .Analytics.RiskAssessment }}
			<div class="risk-score" style="background: {{ riskColor .Level }}; color: white;">{{ .Level }} ({{ printf "%.1f" .Score }}/100)</div>
			{{ if .Issues }}
			<h3>Detected Issues</h3>
			<table>
				<thead><tr><th>Type</th><th>Severity</th><th>Description</th><th>Location</th></tr></thead>
				<tbody>
					{{ range .Issues }}
					<tr>
						<td>{{ .Type }}</td>
						<td><span class="{{ statusClass .Severity }}">{{ .Severity }}</span></td>
						<td>{{ .Description }}</td>
						<td>{{ if .Location }}{{ .Location }}{{ else }}-{{ end }}</td>
					</tr>
					{{ end }}
				</tbody>
			</table>
			{{ end }}
			{{ if .Warnings }}
			<h3>Warnings</h3>
			<div class="alert alert-warning">{{ range .Warnings }}<div>• {{ escapeHTML . }}</div>{{ end }}</div>
			{{ end }}
			{{ end }}
		</section>
		<section id="metadata">
			<h2>Report Information</h2>
			{{ with .Metadata }}
			<table>
				<tr><th>Version</th><td>{{ .GeneratorVersion }}</td></tr>
				<tr><th>Source</th><td>{{ .DataSource }}</td></tr>
				<tr><th>Exported</th><td>{{ formatTime .ExportTime }}</td></tr>
			</table>
			{{ end }}
		</section>
		<footer><p>Generated by ERST Debugging Tools</p></footer>
	</div>
</body>
</html>`
