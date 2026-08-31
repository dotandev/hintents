// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package visualizer

import (
	"bytes"
	_ "embed"
	"strings"
	"text/template"
)

//go:embed hover.tmpl
var hoverTemplateSource string

var hoverTemplate = template.Must(template.New("hover").Parse(hoverTemplateSource))

// hoverContentData is the data passed to hover.tmpl when rendering hover content.
type hoverContentData struct {
	Name        string
	Description string
}

// HostFunctionHoverContent builds markdown-friendly hover content for the given host function.
//
// The presentation (markdown structure) lives in hover.tmpl, embedded at build time so
// documentation formatting can be updated without touching Go code.
func HostFunctionHoverContent(name string) string {
	data := hoverContentData{
		Name:        name,
		Description: DescribeHostFunction(name),
	}

	var buf bytes.Buffer
	if err := hoverTemplate.Execute(&buf, data); err != nil {
		return "**" + name + "**\n\n" + data.Description
	}

	return strings.TrimRight(buf.String(), "\n")
}
