import * as vscode from 'vscode';
import { ERSTClient } from './erstClient';
import { TraceTreeDataProvider, TraceItem } from './traceTreeView';

export function activate(context: vscode.ExtensionContext) {
    const client = new ERSTClient('127.0.0.1', 8080);
    const traceDataProvider = new TraceTreeDataProvider();

    // Register TreeView
    vscode.window.registerTreeDataProvider('erst-traces', traceDataProvider);

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
                }, async (progress) => {
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

        // Show in a virtual document or just a message for PoC
        vscode.workspace.openTextDocument({
            content: stepJson,
            language: 'json'
        }).then(doc => {
            vscode.window.showTextDocument(doc, vscode.ViewColumn.Beside);
        });
    });

    context.subscriptions.push(triggerDebugDisposable, selectTraceStepDisposable, client);
}

export function deactivate() { }
