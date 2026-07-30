// Copyright (c) Hintents Authors.
// SPDX-License-Identifier: Apache-2.0

import * as vscode from 'vscode';
import { ERSTClient } from './erstClient';
import { TraceTreeDataProvider, TraceItem } from './traceTreeView';
import { buildTraceTreeExport, renderStandaloneHtml } from './traceExport';
import { SorobanCodeLensProvider } from './providers/codeLensProvider';
import { TraceHoverProvider } from './providers/hoverProvider';

export function activate(context: vscode.ExtensionContext) {
    const client = new ERSTClient('127.0.0.1', 8080);
    let treeView: vscode.TreeView<vscode.TreeItem> | undefined;
    let traceDataProvider: TraceTreeDataProvider;

    // Register TreeView with provider (pass treeView to provider for auto-reveal)
    traceDataProvider = new TraceTreeDataProvider();
    treeView = vscode.window.createTreeView('erst-traces', { treeDataProvider: traceDataProvider });
    // Patch: set treeView reference in provider for auto-reveal
    (traceDataProvider as any).treeView = treeView;

    // Register TextDocumentContentProvider for states
    const stateProvider = new class implements vscode.TextDocumentContentProvider {
        provideTextDocumentContent(uri: vscode.Uri): string {
            // Decode content from query
            return uri.query;
        }
    };
    context.subscriptions.push(vscode.workspace.registerTextDocumentContentProvider('erst-state', stateProvider));

    const codeLensProvider = new SorobanCodeLensProvider();
    const codeLensDisposable = vscode.languages.registerCodeLensProvider(
        { language: 'rust', scheme: 'file' },
        codeLensProvider
    );

    let runSorobanFunctionDisposable = vscode.commands.registerCommand('erst.runSorobanFunction', async (uri: vscode.Uri, functionName: string) => {
        const document = await vscode.workspace.openTextDocument(uri);
        const selection = new vscode.Selection(0, 0, 0, 0);
        await vscode.window.showTextDocument(document, { preview: false, selection });
        vscode.window.showInformationMessage(`ERST: Ready to simulate ${functionName} from ${document.fileName}`);
    });

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

    // Register hover provider for ERST trace JSON documents
    const hoverProviderDisposable = vscode.languages.registerHoverProvider(
        { scheme: 'file', language: 'json' },
        new TraceHoverProvider(client, traceDataProvider)
    );

    // Navigation: next/prev step commands
    let nextStepDisposable = vscode.commands.registerCommand('erst.nextTraceStep', () => {
        const trace = traceDataProvider.getCurrentTrace();
        if (!trace) return;
        const idx = traceDataProvider.getCurrentStepIndex();
        if (idx < trace.states.length - 1) {
            traceDataProvider.setCurrentStepIndex(idx + 1);
        }
    });
    let prevStepDisposable = vscode.commands.registerCommand('erst.prevTraceStep', () => {
        const trace = traceDataProvider.getCurrentTrace();
        if (!trace) return;
        const idx = traceDataProvider.getCurrentStepIndex();
        if (idx > 0) {
            traceDataProvider.setCurrentStepIndex(idx - 1);
        }
    });

    context.subscriptions.push(
        triggerDebugDisposable,
        selectTraceStepDisposable,
        setSearchQueryDisposable,
        exportTraceTreeDisposable,
        treeView,
        showXdrDisposable,
        showStateDiffDisposable,
        hoverProviderDisposable,
        nextStepDisposable,
        prevStepDisposable,
        codeLensDisposable,
        runSorobanFunctionDisposable,
        client
    );
}

export function deactivate() { }
