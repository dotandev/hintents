// Copyright (c) Hintents Authors.
// SPDX-License-Identifier: Apache-2.0

import { randomBytes } from 'node:crypto';
import * as path from 'node:path';
import * as vscode from 'vscode';

export interface FlamegraphResources {
    cpu: vscode.Uri;
    memory: vscode.Uri;
    title?: string;
}

export class FlamegraphWebview {
    private static currentPanel: vscode.WebviewPanel | undefined;

    public static show(
        resources: FlamegraphResources,
        viewColumn: vscode.ViewColumn = vscode.ViewColumn.Beside
    ): void {
        const localResourceRoots = [
            parentDirectory(resources.cpu),
            parentDirectory(resources.memory)
        ];

        if (FlamegraphWebview.currentPanel) {
            const panel = FlamegraphWebview.currentPanel;
            panel.webview.options = {
                enableScripts: true,
                localResourceRoots
            };
            panel.title = resources.title ?? 'ERST Flamegraphs';
            panel.webview.html = renderWebview(panel.webview, resources);
            panel.reveal(viewColumn, true);
            return;
        }

        const panel = vscode.window.createWebviewPanel(
            'erst.flamegraphs',
            resources.title ?? 'ERST Flamegraphs',
            viewColumn,
            {
                enableScripts: true,
                retainContextWhenHidden: true,
                localResourceRoots
            }
        );

        FlamegraphWebview.currentPanel = panel;
        panel.webview.html = renderWebview(panel.webview, resources);
        panel.onDidDispose(() => {
            FlamegraphWebview.currentPanel = undefined;
        });
    }
}

function renderWebview(
    webview: vscode.Webview,
    resources: FlamegraphResources
): string {
    const nonce = randomBytes(16).toString('hex');
    const cpuSource = escapeAttribute(webview.asWebviewUri(resources.cpu).toString());
    const memorySource = escapeAttribute(webview.asWebviewUri(resources.memory).toString());
    const title = escapeHtml(resources.title ?? 'ERST Flamegraphs');

    return `<!doctype html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta
        http-equiv="Content-Security-Policy"
        content="default-src 'none'; frame-src ${webview.cspSource}; style-src 'nonce-${nonce}'; script-src 'nonce-${nonce}';"
    >
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>${title}</title>
    <style nonce="${nonce}">
        :root {
            color-scheme: light dark;
        }

        body {
            display: flex;
            flex-direction: column;
            height: 100vh;
            margin: 0;
            overflow: hidden;
            color: var(--vscode-foreground);
            background: var(--vscode-editor-background);
            font-family: var(--vscode-font-family);
        }

        .tabs {
            display: flex;
            flex: 0 0 auto;
            gap: 2px;
            padding: 8px 12px 0;
            border-bottom: 1px solid var(--vscode-panel-border);
            background: var(--vscode-editorGroupHeader-tabsBackground);
        }

        .tab {
            min-width: 96px;
            padding: 8px 14px;
            border: 0;
            border-bottom: 2px solid transparent;
            color: var(--vscode-tab-inactiveForeground);
            background: transparent;
            cursor: pointer;
            font: inherit;
        }

        .tab:hover {
            color: var(--vscode-tab-activeForeground);
            background: var(--vscode-list-hoverBackground);
        }

        .tab[aria-selected="true"] {
            color: var(--vscode-tab-activeForeground);
            border-bottom-color: var(--vscode-focusBorder);
            background: var(--vscode-tab-activeBackground);
        }

        .tab:focus-visible {
            outline: 1px solid var(--vscode-focusBorder);
            outline-offset: -2px;
        }

        .graphs {
            position: relative;
            flex: 1 1 auto;
            min-height: 0;
        }

        .graph {
            position: absolute;
            inset: 0;
        }

        .graph[hidden] {
            display: none;
        }

        iframe {
            width: 100%;
            height: 100%;
            border: 0;
            background: var(--vscode-editor-background);
        }
    </style>
</head>
<body>
    <nav class="tabs" role="tablist" aria-label="Flamegraph resource">
        <button
            class="tab"
            id="cpu-tab"
            type="button"
            role="tab"
            aria-selected="true"
            aria-controls="cpu-graph"
            data-target="cpu-graph"
        >
            CPU
        </button>
        <button
            class="tab"
            id="memory-tab"
            type="button"
            role="tab"
            aria-selected="false"
            aria-controls="memory-graph"
            data-target="memory-graph"
        >
            Memory
        </button>
    </nav>
    <main class="graphs">
        <section
            class="graph"
            id="cpu-graph"
            role="tabpanel"
            aria-labelledby="cpu-tab"
        >
            <iframe
                title="CPU flamegraph"
                src="${cpuSource}"
                sandbox="allow-scripts allow-same-origin"
            ></iframe>
        </section>
        <section
            class="graph"
            id="memory-graph"
            role="tabpanel"
            aria-labelledby="memory-tab"
            hidden
        >
            <iframe
                title="Memory flamegraph"
                src="${memorySource}"
                sandbox="allow-scripts allow-same-origin"
            ></iframe>
        </section>
    </main>
    <script nonce="${nonce}">
        const tabs = Array.from(document.querySelectorAll('[role="tab"]'));
        const panels = Array.from(document.querySelectorAll('[role="tabpanel"]'));

        function activateTab(tab) {
            const target = tab.dataset.target;

            for (const candidate of tabs) {
                candidate.setAttribute(
                    'aria-selected',
                    String(candidate === tab)
                );
            }

            for (const panel of panels) {
                panel.hidden = panel.id !== target;
            }
        }

        for (const tab of tabs) {
            tab.addEventListener('click', () => activateTab(tab));
            tab.addEventListener('keydown', (event) => {
                if (event.key !== 'ArrowLeft' && event.key !== 'ArrowRight') {
                    return;
                }

                event.preventDefault();
                const direction = event.key === 'ArrowRight' ? 1 : -1;
                const currentIndex = tabs.indexOf(tab);
                const nextIndex = (currentIndex + direction + tabs.length) % tabs.length;
                const nextTab = tabs[nextIndex];
                nextTab.focus();
                activateTab(nextTab);
            });
        }
    </script>
</body>
</html>`;
}

function parentDirectory(uri: vscode.Uri): vscode.Uri {
    return vscode.Uri.file(path.dirname(uri.fsPath));
}

function escapeAttribute(value: string): string {
    return escapeHtml(value);
}

function escapeHtml(value: string): string {
    return value
        .replaceAll('&', '&amp;')
        .replaceAll('<', '&lt;')
        .replaceAll('>', '&gt;')
        .replaceAll('"', '&quot;')
        .replaceAll("'", '&#39;');
}
