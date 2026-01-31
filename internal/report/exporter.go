// Copyright 2026 dotandev
// SPDX-License-Identifier: Apache-2.0

package report

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type Exporter struct {
	outputDir string
}

func NewExporter(outputDir string) (*Exporter, error) {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	return &Exporter{outputDir: outputDir}, nil
}

func (e *Exporter) Export(report *Report, format string) (string, error) {
	filename := generateFilename(report.Title, format)
	filepath := filepath.Join(e.outputDir, filename)

	var data []byte
	var err error

	switch strings.ToLower(format) {
	case "json":
		data, err = json.MarshalIndent(report, "", "  ")
	case "html":
		renderer := NewHTMLRenderer()
		data, err = renderer.Render(report)
	case "pdf":
		renderer := NewPDFRenderer()
		data, err = renderer.Render(report)
	default:
		return "", fmt.Errorf("unsupported format: %s", format)
	}

	if err != nil {
		return "", fmt.Errorf("failed to render report: %w", err)
	}

	if err := os.WriteFile(filepath, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	return filepath, nil
}

func (e *Exporter) ExportMultiple(report *Report, formats []string) (map[string]string, error) {
	results := make(map[string]string)

	for _, format := range formats {
		path, err := e.Export(report, format)
		if err != nil {
			return results, fmt.Errorf("failed to export %s: %w", format, err)
		}
		results[format] = path
	}

	return results, nil
}

func generateFilename(title string, format string) string {
	sanitized := sanitizeFilename(title)
	timestamp := time.Now().Format("20060102-150405")
	return fmt.Sprintf("%s-%s.%s", sanitized, timestamp, format)
}

func sanitizeFilename(name string) string {
	reg := regexp.MustCompile("[^a-zA-Z0-9-_]")
	sanitized := reg.ReplaceAllString(name, "_")
	sanitized = strings.ToLower(sanitized)

	for strings.Contains(sanitized, "__") {
		sanitized = strings.ReplaceAll(sanitized, "__", "_")
	}

	if len(sanitized) > 50 {
		sanitized = sanitized[:50]
	}

	return sanitized
}
