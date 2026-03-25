// Copyright (c) Hintents Authors.
// SPDX-License-Identifier: Apache-2.0

import * as vscode from 'vscode';
import { ERSTClient } from './erstClient';
import { TraceTreeDataProvider, TraceItem } from './traceTreeView';
import { buildTraceTreeExport, renderStandaloneHtml } from './traceExport';

export function activate(context: vscode.ExtensionContext) {
    const client = new ERSTClient('127.0.0.1', 8080);
    const traceDataProvider = new TraceTreeDataProvider();

    // Register TreeView
    const treeView = vscode.window.createTreeView('erst-traces', { treeDataProvider: traceDataProvider });

    // Register TextDocumentContentProvider for states
    const stateProvider = new class implements vscode.TextDocumentContentProvider {
        provideTextDocumentContent(uri: vscode.Uri): string {
            // Decode content from query
            return uri.query;
        }
    };
    context.subscriptions.push(vscode.workspace.registerTextDocumentContentProvider('erst-state', stateProvider));

    // Register command: erst.triggerDebug
    let triggerDebugDisposable = vscode.commands.registerCommand('erst.triggerDebug', async () => {
        const hash = await vscode.window.showInputBox({
            prompt: 'Enter Transaction Hash to Debug',
            placeHolder: 'e.g., sample-tx-hash-1234'
        });

        if (hash) {
            try {
                await vscode.window.withProgress({
                    location: vscode.ProgressLocation.Notification,
                    title: "ERST: Debugging Transaction...",
                    cancellable: false
                }, async (progress: vscode.Progress<{ message?: string; increment?: number }>) => {
                    await client.connect();
                    await client.debugTransaction(hash);
                    const trace = await client.getTrace(hash);
                    traceDataProvider.refresh(trace);
                });
                vscode.window.showInformationMessage(`Trace loaded for ${hash}`);
            } catch (err: any) {
                vscode.window.showErrorMessage(`ERST Error: ${err.message}`);
            }
        }
    });

    // Handle selecting a trace item
    let selectTraceStepDisposable = vscode.commands.registerCommand('erst.selectTraceStep', (item: TraceItem) => {
        const stepJson = JSON.stringify(item.step, null, 2);

        vscode.workspace.openTextDocument({
            content: stepJson,
            language: 'json'
        }).then((doc: vscode.TextDocument) => {
            vscode.window.showTextDocument(doc, vscode.ViewColumn.Beside);
        });
    });

    let setSearchQueryDisposable = vscode.commands.registerCommand('erst.setTraceSearchQuery', async () => {
        const value = await vscode.window.showInputBox({
            prompt: 'Set trace search query for export matching',
            placeHolder: 'e.g., transfer or contract-id prefix',
            value: traceDataProvider.getSearchQuery()
        });

        if (value !== undefined) {
            traceDataProvider.setSearchQuery(value);
            const label = value.trim() === '' ? '(cleared)' : `"${value}"`;
            vscode.window.showInformationMessage(`Trace search query updated: ${label}`);
        }
    });

    let exportTraceTreeDisposable = vscode.commands.registerCommand('erst.exportTraceTree', async () => {
        const trace = traceDataProvider.getCurrentTrace();
        if (!trace) {
            vscode.window.showWarningMessage('Load a trace first, then export.');
            return;
        }

        const defaultBase = `${trace.transaction_hash || 'trace'}-trace-tree.html`;
        const defaultDir =
            vscode.workspace.workspaceFolders?.[0]?.uri ?? context.globalStorageUri;
        const defaultUri = vscode.Uri.joinPath(defaultDir, defaultBase);
        const htmlTarget = await vscode.window.showSaveDialog({
            title: 'Export trace tree as standalone HTML',
            defaultUri,
            filters: { HTML: ['html'] }
        });

        if (!htmlTarget) {
            return;
        }

        const payload = buildTraceTreeExport(trace, traceDataProvider.getSearchQuery());
        const html = renderStandaloneHtml(payload);
        const json = JSON.stringify(payload, null, 2);
        const jsonPath = htmlTarget.fsPath.replace(/\.html?$/i, '.json');
        const jsonTarget = vscode.Uri.file(jsonPath);

        await vscode.workspace.fs.writeFile(htmlTarget, Buffer.from(html, 'utf8'));
        await vscode.workspace.fs.writeFile(jsonTarget, Buffer.from(json, 'utf8'));

        vscode.window.showInformationMessage(
            `Trace tree exported: ${htmlTarget.fsPath} and ${jsonTarget.fsPath}`
        );
    });

    // Handle showing XDR
    let showXdrDisposable = vscode.commands.registerCommand('erst.showXdr', (xdr: string) => {
        vscode.workspace.openTextDocument({
            content: xdr,
            language: 'text'
        }).then((doc: vscode.TextDocument) => {
            vscode.window.showTextDocument(doc, vscode.ViewColumn.Beside);
        });
    });

    // Handle showing state diff
    let showStateDiffDisposable = vscode.commands.registerCommand('erst.showStateDiff', (before: string, after: string) => {
        const baseUri = vscode.Uri.parse('erst-state:state');
        const beforeUri = baseUri.with({ path: 'before', query: before });
        const afterUri = baseUri.with({ path: 'after', query: after });

        vscode.commands.executeCommand('vscode.diff', beforeUri, afterUri, 'State Diff (Before vs After)');
    });

    context.subscriptions.push(
        triggerDebugDisposable,
        selectTraceStepDisposable,
        setSearchQueryDisposable,
        exportTraceTreeDisposable,
        treeView,
        showXdrDisposable,
        showStateDiffDisposable,
        treeView,
        client
    );
}

export function deactivate() { }
