import { Trace, TraceStep } from './erstClient';

export interface ExportNode {
    id: string;
    label: string;
    description?: string;
    matched: boolean;
    children: ExportNode[];
}

export interface TraceTreeExport {
    version: string;
    exportedAt: string;
    transactionHash: string;
    searchQuery: string;
    totalMatches: number;
    matchedNodeIds: string[];
    tree: ExportNode[];
}

export function buildTraceTreeExport(trace: Trace, searchQuery: string): TraceTreeExport {
    const query = searchQuery.trim().toLowerCase();
    const roots = trace.states.map((step) => stepToNode(step));
    const matchedNodeIds: string[] = [];

    const markMatches = (node: ExportNode): void => {
        node.matched = query.length > 0 && isNodeMatch(node, query);
        if (node.matched) {
            matchedNodeIds.push(node.id);
        }
        for (const child of node.children) {
            markMatches(child);
        }
    };

    for (const root of roots) {
        markMatches(root);
    }

    return {
        version: '1.0.0',
        exportedAt: new Date().toISOString(),
        transactionHash: trace.transaction_hash,
        searchQuery,
        totalMatches: matchedNodeIds.length,
        matchedNodeIds,
        tree: roots
    };
}

export function renderStandaloneHtml(payload: TraceTreeExport): string {
    const escapedHash = escapeHtml(payload.transactionHash);
    const escapedQuery = escapeHtml(payload.searchQuery);
    const rows = payload.tree.map((n) => renderNodeHtml(n)).join('\n');
    return `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>ERST Trace Tree Export</title>
  <style>
    :root {
      --bg: #f4f7f2;
      --text: #1a2d24;
      --muted: #4a6056;
      --panel: #ffffff;
      --line: #d0dbd4;
      --accent: #0b6e4f;
      --match: #fff3bf;
    }
    body {
      margin: 0;
      padding: 24px;
      font-family: "Segoe UI", "Helvetica Neue", Arial, sans-serif;
      background: radial-gradient(circle at top, #eef6f1 0%, var(--bg) 60%);
      color: var(--text);
    }
    .card {
      max-width: 1100px;
      margin: 0 auto;
      background: var(--panel);
      border: 1px solid var(--line);
      border-radius: 12px;
      padding: 18px 22px;
      box-shadow: 0 8px 28px rgba(20, 40, 32, 0.08);
    }
    h1 {
      margin: 0 0 10px;
      color: var(--accent);
      font-size: 26px;
    }
    .meta {
      margin-bottom: 16px;
      color: var(--muted);
      font-size: 14px;
      display: grid;
      gap: 4px;
    }
    .tree {
      border-top: 1px solid var(--line);
      margin-top: 12px;
      padding-top: 12px;
    }
    details {
      margin: 4px 0 4px 14px;
      border-left: 1px dotted var(--line);
      padding-left: 10px;
    }
    summary {
      cursor: pointer;
      list-style: none;
      padding: 2px 0;
    }
    summary::-webkit-details-marker {
      display: none;
    }
    .label {
      font-weight: 600;
    }
    .desc {
      color: var(--muted);
      margin-left: 6px;
      font-size: 13px;
    }
    .match {
      background: var(--match);
      border-radius: 5px;
      padding: 1px 4px;
      box-decoration-break: clone;
      -webkit-box-decoration-break: clone;
    }
    .leaf {
      margin: 4px 0 4px 28px;
    }
    pre {
      margin-top: 18px;
      background: #f8fbf9;
      border: 1px solid var(--line);
      border-radius: 10px;
      padding: 12px;
      overflow: auto;
      font-size: 12px;
      line-height: 1.45;
    }
  </style>
</head>
<body>
  <main class="card">
    <h1>ERST Trace Tree Export</h1>
    <section class="meta">
      <div><strong>Transaction:</strong> ${escapedHash}</div>
      <div><strong>Exported:</strong> ${escapeHtml(payload.exportedAt)}</div>
      <div><strong>Search query:</strong> ${escapedQuery || '(none)'}</div>
      <div><strong>Matched nodes:</strong> ${payload.totalMatches}</div>
    </section>
    <section class="tree">
${rows}
    </section>
    <pre id="payload">${escapeHtml(JSON.stringify(payload, null, 2))}</pre>
  </main>
</body>
</html>`;
}

function stepToNode(step: TraceStep): ExportNode {
    const stepId = `step-${step.step}`;
    const children: ExportNode[] = [];

    children.push(textNode(`${stepId}-timestamp`, 'timestamp', step.timestamp));
    children.push(textNode(`${stepId}-operation`, 'operation', step.operation));
    if (step.contract_id) {
        children.push(textNode(`${stepId}-contract_id`, 'contract_id', step.contract_id));
    }
    if (step.function) {
        children.push(textNode(`${stepId}-function`, 'function', step.function));
    }
    if (step.arguments !== undefined) {
        children.push(valueNode(`${stepId}-arguments`, 'arguments', step.arguments));
    }
    if (step.return_value !== undefined) {
        children.push(valueNode(`${stepId}-return_value`, 'return_value', step.return_value));
    }
    if (step.error) {
        children.push(textNode(`${stepId}-error`, 'error', step.error));
    }
    if (step.host_state !== undefined) {
        children.push(valueNode(`${stepId}-host_state`, 'host_state', step.host_state));
    }
    if (step.memory !== undefined) {
        children.push(valueNode(`${stepId}-memory`, 'memory', step.memory));
    }

    return {
        id: stepId,
        label: `step ${step.step}: ${step.operation}`,
        description: step.function ? `function=${step.function}` : undefined,
        matched: false,
        children
    };
}

function textNode(id: string, label: string, value: string): ExportNode {
    return {
        id,
        label,
        description: value,
        matched: false,
        children: []
    };
}

function valueNode(id: string, label: string, value: unknown): ExportNode {
    if (value === null) {
        return textNode(id, label, 'null');
    }

    if (Array.isArray(value)) {
        const children = value.map((entry, index) => valueNode(`${id}-${index}`, `[${index}]`, entry));
        return {
            id,
            label,
            description: `array(${value.length})`,
            matched: false,
            children
        };
    }

    if (typeof value === 'object') {
        const objectValue = value as Record<string, unknown>;
        const keys = Object.keys(objectValue);
        const children = keys.map((key) => valueNode(`${id}-${sanitizeId(key)}`, key, objectValue[key]));
        return {
            id,
            label,
            description: `object(${keys.length})`,
            matched: false,
            children
        };
    }

    return textNode(id, label, String(value));
}

function sanitizeId(value: string): string {
    return value.replace(/[^a-zA-Z0-9_-]/g, '_');
}

function isNodeMatch(node: ExportNode, query: string): boolean {
    const haystack = `${node.label} ${node.description ?? ''}`.toLowerCase();
    return haystack.includes(query);
}

function renderNodeHtml(node: ExportNode): string {
    const labelClass = node.matched ? 'label match' : 'label';
    const summary = `<span class="${labelClass}">${escapeHtml(node.label)}</span>${
        node.description ? `<span class="desc">${escapeHtml(node.description)}</span>` : ''
    }`;

    if (node.children.length === 0) {
        return `      <div class="leaf">${summary}</div>`;
    }

    const children = node.children.map((child) => renderNodeHtml(child)).join('\n');
    return `      <details open>
        <summary>${summary}</summary>
${children}
      </details>`;
}

function escapeHtml(input: string): string {
    return input
        .replaceAll('&', '&amp;')
        .replaceAll('<', '&lt;')
        .replaceAll('>', '&gt;')
        .replaceAll('"', '&quot;')
        .replaceAll("'", '&#39;');
}
