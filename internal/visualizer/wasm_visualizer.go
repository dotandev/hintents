// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package visualizer

import (
	"fmt"
	"strings"

	"github.com/dotandev/hintents/internal/simulator"
)

// GenerateWasmReport generates an HTML report visualizing WASM section sizes
func GenerateWasmReport(analysis *simulator.WasmAnalysis) string {
	var sectionsJS strings.Builder
	for i, s := range analysis.Sections {
		sectionsJS.WriteString(fmt.Sprintf("{name: '%s', size: %d, category: '%s'}", s.Name, s.Size, s.Category))
		if i < len(analysis.Sections)-1 {
			sectionsJS.WriteString(",")
		}
	}

	html := `
<!DOCTYPE html>
<html>
<head>
    <title>WASM Analysis Report</title>
    <style>
        body { font-family: 'Inter', system-ui, -apple-system, sans-serif; background: #0f172a; color: #f8fafc; margin: 0; padding: 2rem; }
        .container { max-width: 1000px; margin: 0 auto; }
        h1 { color: #38bdf8; border-bottom: 2px solid #334155; padding-bottom: 1rem; }
        .summary { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 1rem; margin-bottom: 2rem; }
        .card { background: #1e293b; padding: 1.5rem; border-radius: 0.75rem; box-shadow: 0 4px 6px -1px rgb(0 0 0 / 0.1); border: 1px solid #334155; }
        .card h3 { margin: 0; color: #94a3b8; font-size: 0.875rem; text-transform: uppercase; }
        .card .value { font-size: 1.875rem; font-weight: 700; margin-top: 0.5rem; }
        
        .visualizer { height: 40px; display: flex; border-radius: 0.5rem; overflow: hidden; margin-bottom: 2rem; border: 1px solid #334155; }
        .bar { height: 100%; transition: width 0.3s ease; }
        
        table { width: 100%; border-collapse: collapse; background: #1e293b; border-radius: 0.75rem; overflow: hidden; border: 1px solid #334155; }
        th { text-align: left; padding: 1rem; background: #334155; color: #94a3b8; font-size: 0.75rem; text-transform: uppercase; }
        td { padding: 1rem; border-bottom: 1px solid #334155; }
        tr:last-child td { border-bottom: none; }
        
        .cat-Logic { background: #3b82f6; }
        .cat-DebugInfo { background: #10b981; }
        .cat-Data { background: #f59e0b; }
        .cat-Other { background: #64748b; }
        
        .source-files { margin-top: 2rem; }
        .source-files ul { list-style: none; padding: 0; display: grid; grid-template-columns: repeat(auto-fill, minmax(200px, 1fr)); gap: 0.5rem; }
        .source-files li { background: #1e293b; padding: 0.75rem; border-radius: 0.5rem; border: 1px solid #334155; display: flex; align-items: center; gap: 0.5rem; }
        .source-icon { color: #94a3b8; }

        .legend { display: flex; gap: 1rem; margin-bottom: 1rem; justify-content: flex-end; }
        .legend-item { display: flex; align-items: center; gap: 0.5rem; font-size: 0.875rem; }
        .dot { width: 12px; height: 12px; border-radius: 50%; }
    </style>
</head>
<body>
    <div class="container">
        <h1>WASM Binary Analysis</h1>
        
        <div class="summary">
            <div class="card">
                <h3>Total Size</h3>
                <div class="value">` + fmt.Sprintf("%d", analysis.TotalSize) + ` <span style="font-size: 1rem">bytes</span></div>
            </div>
            <div class="card">
                <h3>Sections</h3>
                <div class="value">` + fmt.Sprintf("%d", len(analysis.Sections)) + `</div>
            </div>
        </div>

        <div class="legend">
            <div class="legend-item"><div class="dot cat-Logic"></div> Logic</div>
            <div class="legend-item"><div class="dot cat-DebugInfo"></div> Debug Info</div>
            <div class="legend-item"><div class="dot cat-Data"></div> Data</div>
            <div class="legend-item"><div class="dot cat-Other"></div> Other</div>
        </div>

        <div class="visualizer" id="visualizer"></div>

        <div id="source-section" class="source-files" style="display: none;">
            <h3>Source File Mappings</h3>
            <ul id="source-list"></ul>
        </div>

        <table style="margin-top: 2rem;">
            <thead>
                <tr>
                    <th>Section Name</th>
                    <th>Category</th>
                    <th>Size (Bytes)</th>
                    <th>Percentage</th>
                </tr>
            </thead>
            <tbody id="table-body"></tbody>
        </table>
    </div>

    <script>
        const sections = [` + sectionsJS.String() + `];
        const sourceFiles = [` + formatSourceFiles(analysis.SourceFiles) + `];
        const totalSize = ` + fmt.Sprintf("%d", analysis.TotalSize) + `;
        
        const visualizer = document.getElementById('visualizer');
        const tableBody = document.getElementById('table-body');
        const sourceSection = document.getElementById('source-section');
        const sourceList = document.getElementById('source-list');
        
        const catTotals = { Logic: 0, DebugInfo: 0, Data: 0, Other: 0 };
        sections.forEach(s => {
            if (catTotals[s.category] !== undefined) {
                catTotals[s.category] += s.size;
            } else {
                catTotals['Other'] += s.size;
            }
        });
        
        // Show source files if present
        if (sourceFiles.length > 0) {
            sourceSection.style.display = 'block';
            sourceFiles.forEach(f => {
                const li = document.createElement('li');
                li.innerHTML = '<span class="source-icon"></span> ' + f;
                sourceList.appendChild(li);
            });
        }

        // Create visualizer bars
        Object.keys(catTotals).forEach(cat => {
            if (catTotals[cat] > 0) {
                const bar = document.createElement('div');
                bar.className = 'bar cat-' + cat;
                bar.style.width = ((catTotals[cat] / totalSize) * 100) + '%';
                bar.title = cat + ': ' + catTotals[cat] + ' bytes';
                visualizer.appendChild(bar);
            }
        });
        
        // Populate table
        sections.sort((a, b) => b.size - a.size).forEach(s => {
            const row = document.createElement('tr');
            row.innerHTML = ` + "`" + `
                <td>${s.name}</td>
                <td><span style="display: flex; align-items: center; gap: 0.5rem;"><div class="dot cat-${s.category}"></div> ${s.category}</span></td>
                <td>${s.size.toLocaleString()}</td>
                <td>${((s.size / totalSize) * 100).toFixed(2)}%</td>
            ` + "`" + `;
            tableBody.appendChild(row);
        });
    </script>
</body>
</html>
`
	return html
}

func formatSourceFiles(files []string) string {
	if len(files) == 0 {
		return ""
	}
	quoted := make([]string, len(files))
	for i, f := range files {
		quoted[i] = fmt.Sprintf("'%s'", f)
	}
	return strings.Join(quoted, ",")
}
