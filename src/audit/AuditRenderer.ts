// Copyright (c) 2026 dotandev
// SPDX-License-Identifier: MIT OR Apache-2.0

import { writeFileSync } from 'fs';
import type { ExecutionTrace, SignedAuditLog } from './AuditLogger';

export type AuditPayload = ExecutionTrace | SignedAuditLog;

function isSigned(payload: AuditPayload): payload is SignedAuditLog {
  return 'trace' in payload && 'hash' in payload;
}

function escapeHtml(value: string): string {
  return value
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;');
}

function renderKeyValueTable(data: Record<string, any>): string {
  const rows = Object.entries(data)
    .map(([k, v]) => {
      const val = typeof v === 'object' ? JSON.stringify(v) : String(v);
      return `<tr><td class="key">${escapeHtml(k)}</td><td class="val"><code>${escapeHtml(val)}</code></td></tr>`;
    })
    .join('\n');
  return `<table><thead><tr><th>Key</th><th>Value</th></tr></thead><tbody>${rows}</tbody></table>`;
}

function renderEventsList(events: any[]): string {
  if (events.length === 0) {
    return '<p class="empty">No events recorded.</p>';
  }
  const items = events
    .map((e, i) => {
      const label = typeof e === 'string' ? escapeHtml(e) : escapeHtml(JSON.stringify(e));
      return `<li><span class="event-index">${i + 1}</span>${label}</li>`;
    })
    .join('\n');
  return `<ol class="events-list">${items}</ol>`;
}

function renderSignatureSection(log: SignedAuditLog): string {
  const truncatedKey = log.publicKey.length > 80
    ? log.publicKey.slice(0, 40) + '...' + log.publicKey.slice(-40)
    : log.publicKey;
  return `
    <table>
      <tr><th>Algorithm</th><td><code>${escapeHtml(log.algorithm)}</code></td></tr>
      <tr><th>Hash (SHA-256)</th><td><code>${escapeHtml(log.hash)}</code></td></tr>
      <tr><th>Signature</th><td><code>${escapeHtml(log.signature.slice(0, 32))}...</code></td></tr>
      <tr><th>Public Key</th><td><code>${escapeHtml(truncatedKey)}</code></td></tr>
      <tr><th>Signer Provider</th><td>${escapeHtml(log.signer.provider)}</td></tr>
    </table>`;
}

/**
 * Renders a raw ExecutionTrace or SignedAuditLog as a self-contained HTML document.
 */
export function renderAuditHTML(payload: AuditPayload, title?: string): string {
  const signed = isSigned(payload);
  const trace: ExecutionTrace = signed ? payload.trace : (payload as ExecutionTrace);
  const reportTitle = title ?? 'Audit Report';
  const generatedAt = trace.timestamp
    ? new Date(trace.timestamp).toUTCString()
    : new Date().toUTCString();

  const inputSection = Object.keys(trace.input).length > 0
    ? renderKeyValueTable(trace.input)
    : '<p class="empty">No input data.</p>';

  const stateSection = Object.keys(trace.state).length > 0
    ? renderKeyValueTable(trace.state)
    : '<p class="empty">No state data.</p>';

  const eventsSection = renderEventsList(trace.events);

  const signatureBlock = signed
    ? `<section id="signature"><h2>Signature &amp; Integrity</h2>${renderSignatureSection(payload as SignedAuditLog)}</section>`
    : '';

  const tocSignature = signed
    ? '<a href="#signature">Signature</a>'
    : '';

  return `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>${escapeHtml(reportTitle)}</title>
  <style>
    * { margin: 0; padding: 0; box-sizing: border-box; }
    body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; color: #333; background: #f5f5f5; line-height: 1.6; }
    .container { max-width: 1100px; margin: 0 auto; background: #fff; box-shadow: 0 0 10px rgba(0,0,0,0.08); }
    header { background: linear-gradient(135deg, #1a1a2e 0%, #16213e 100%); color: #fff; padding: 36px 32px; }
    header h1 { font-size: 2em; margin-bottom: 6px; }
    .header-meta { font-size: 0.85em; opacity: 0.75; }
    .toc { background: #f9f9f9; border-bottom: 1px solid #e0e0e0; padding: 14px 32px; display: flex; gap: 24px; flex-wrap: wrap; }
    .toc a { color: #1a1a2e; text-decoration: none; font-weight: 500; font-size: 0.9em; }
    .toc a:hover { text-decoration: underline; }
    section { padding: 36px 32px; border-bottom: 1px solid #e8e8e8; }
    section:last-child { border-bottom: none; }
    h2 { font-size: 1.4em; color: #1a1a2e; margin-bottom: 18px; padding-bottom: 8px; border-bottom: 2px solid #1a1a2e; }
    table { width: 100%; border-collapse: collapse; margin-top: 8px; font-size: 0.92em; }
    thead { background: #f0f0f0; }
    th { padding: 10px 14px; text-align: left; font-weight: 600; color: #555; }
    td { padding: 9px 14px; border-bottom: 1px solid #ebebeb; vertical-align: top; }
    td.key { font-weight: 500; white-space: nowrap; width: 30%; color: #444; }
    td.val { word-break: break-all; }
    tbody tr:hover { background: #fafafa; }
    code { font-family: "SFMono-Regular", Consolas, monospace; font-size: 0.88em; background: #f4f4f4; padding: 1px 4px; border-radius: 3px; }
    .events-list { list-style: none; padding: 0; margin-top: 4px; }
    .events-list li { display: flex; align-items: flex-start; gap: 10px; padding: 8px 0; border-bottom: 1px solid #f0f0f0; font-size: 0.93em; }
    .events-list li:last-child { border-bottom: none; }
    .event-index { min-width: 28px; height: 24px; background: #1a1a2e; color: #fff; border-radius: 4px; font-size: 0.75em; font-weight: 700; display: flex; align-items: center; justify-content: center; flex-shrink: 0; margin-top: 1px; }
    .empty { color: #999; font-style: italic; font-size: 0.9em; }
    footer { background: #f5f5f5; padding: 18px 32px; text-align: center; color: #aaa; font-size: 0.82em; border-top: 1px solid #e8e8e8; }
    @media print { body { background: #fff; } .container { box-shadow: none; } section { page-break-inside: avoid; } }
  </style>
</head>
<body>
  <div class="container">
    <header>
      <h1>${escapeHtml(reportTitle)}</h1>
      <div class="header-meta">Generated: ${escapeHtml(generatedAt)}</div>
    </header>
    <nav class="toc">
      <a href="#input">Input</a>
      <a href="#state">State</a>
      <a href="#events">Events</a>
      ${tocSignature}
    </nav>
    <section id="input">
      <h2>Input</h2>
      ${inputSection}
    </section>
    <section id="state">
      <h2>State</h2>
      ${stateSection}
    </section>
    <section id="events">
      <h2>Events (${trace.events.length})</h2>
      ${eventsSection}
    </section>
    ${signatureBlock}
    <footer>Generated by ERST Audit Tools</footer>
  </div>
</body>
</html>`;
}

/**
 * Renders a raw ExecutionTrace or SignedAuditLog to an HTML file.
 */
export function writeAuditReport(payload: AuditPayload, outputPath: string, title?: string): void {
  const html = renderAuditHTML(payload, title);
  writeFileSync(outputPath, html, 'utf8');
}
