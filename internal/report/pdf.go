// Copyright 2026 dotandev
// SPDX-License-Identifier: Apache-2.0

package report

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

type PDFRenderer struct {
	htmlRenderer *HTMLRenderer
}

func NewPDFRenderer() *PDFRenderer {
	return &PDFRenderer{
		htmlRenderer: NewHTMLRenderer(),
	}
}

func (p *PDFRenderer) Render(report *Report) ([]byte, error) {
	html, err := p.htmlRenderer.Render(report)
	if err != nil {
		return nil, fmt.Errorf("failed to generate HTML: %w", err)
	}

	return convertHTMLToPDF(html, report.Title)
}

func convertHTMLToPDF(htmlContent []byte, title string) ([]byte, error) {
	tmpHTML := filepath.Join(os.TempDir(), "report-"+randomID()+".html")
	tmpPDF := filepath.Join(os.TempDir(), "report-"+randomID()+".pdf")

	defer os.Remove(tmpHTML)
	defer os.Remove(tmpPDF)

	if err := os.WriteFile(tmpHTML, htmlContent, 0644); err != nil {
		return nil, fmt.Errorf("failed to write temp HTML: %w", err)
	}

	if err := convertWithWkhtmltopdf(tmpHTML, tmpPDF); err == nil {
		return os.ReadFile(tmpPDF)
	}

	return generateEmbeddedPDF(htmlContent, title)
}

func convertWithWkhtmltopdf(htmlPath, pdfPath string) error {
	cmd := exec.Command("wkhtmltopdf", "--quiet", htmlPath, pdfPath)
	cmd.Stdout = &bytes.Buffer{}
	cmd.Stderr = &bytes.Buffer{}
	return cmd.Run()
}

func generateEmbeddedPDF(htmlContent []byte, title string) ([]byte, error) {
	pdf := createMinimalPDF(title, string(htmlContent))
	return []byte(pdf), nil
}

func createMinimalPDF(title, content string) string {
	timestamp := time.Now().Unix()

	header := fmt.Sprintf(`%%PDF-1.4
1 0 obj
<< /Type /Catalog /Pages 2 0 R >>
endobj
2 0 obj
<< /Type /Pages /Kids [3 0 R] /Count 1 >>
endobj
3 0 obj
<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Contents 4 0 R /Resources << /Font << /F1 5 0 R >> >> >>
endobj
4 0 obj
<< /Length %d >>
stream
BT
/F1 12 Tf
50 700 Td
(%s) Tj
0 -20 Td
(Generated: %d) Tj
ET
endstream
endobj
5 0 obj
<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>
endobj
xref
0 6
0000000000 65535 f 
0000000009 00000 n 
0000000058 00000 n 
0000000115 00000 n 
0000000214 00000 n 
0000000359 00000 n 
trailer
<< /Size 6 /Root 1 0 R >>
startxref
434
%%%%EOF`, len(content)+100, title, timestamp)

	return header
}

func randomID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
