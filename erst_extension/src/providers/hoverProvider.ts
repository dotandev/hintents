// Copyright (c) Hintents Authors.
// SPDX-License-Identifier: Apache-2.0

import * as vscode from 'vscode';
import { ERSTClient, Trace, TraceStep } from '../erstClient';
import { TraceTreeDataProvider } from '../traceTreeView';

export class TraceHoverProvider implements vscode.HoverProvider {
    constructor(
        private readonly client: ERSTClient,
        private readonly traceDataProvider: TraceTreeDataProvider
    ) {}

    public async provideHover(
        document: vscode.TextDocument,
        position: vscode.Position,
        token: vscode.CancellationToken
    ): Promise<vscode.Hover | null> {
        const currentTrace = this.traceDataProvider.getCurrentTrace();
        if (!currentTrace) {
            return null;
        }

        const wordRange = document.getWordRangeAtPosition(position, /[A-Za-z0-9_]+/);
        if (!wordRange) {
            return null;
        }

        const variableName = document.getText(wordRange).trim();
        if (!variableName) {
            return null;
        }

        let trace: Trace = currentTrace;
        try {
            trace = await this.client.getTrace(currentTrace.transaction_hash);
        } catch {
            // Fall back to in-memory trace if ERST is unavailable.
        }

        const currentStep = trace.states[this.traceDataProvider.getCurrentStepIndex()];
        if (!currentStep) {
            return null;
        }

        const matchedValue = findVariableValue(variableName, currentStep);
        if (matchedValue === undefined) {
            return null;
        }

        const markdown = new vscode.MarkdownString();
        markdown.appendMarkdown(`**ERST value for** \\`${variableName}\\`\n\n`);
        markdown.appendCodeblock(JSON.stringify(matchedValue, null, 2), 'json');
        markdown.isTrusted = true;

        return new vscode.Hover(markdown, wordRange);
    }
}

function findVariableValue(variableName: string, step: TraceStep): unknown {
    const candidates = [
        step.host_state?.after,
        step.host_state?.before,
        step.arguments,
        step.return_value,
        { error: step.error }
    ];

    for (const candidate of candidates) {
        if (candidate === undefined) {
            continue;
        }

        const found = searchValue(variableName, candidate, new Set());
        if (found !== undefined) {
            return found;
        }
    }

    return undefined;
}

function searchValue(variableName: string, value: unknown, visited: Set<unknown>): unknown | undefined {
    if (value === null || typeof value !== 'object') {
        return undefined;
    }

    if (visited.has(value)) {
        return undefined;
    }
    visited.add(value);

    if (Array.isArray(value)) {
        for (const item of value) {
            const found = searchValue(variableName, item, visited);
            if (found !== undefined) {
                return found;
            }
        }
        return undefined;
    }

    for (const [key, child] of Object.entries(value as Record<string, unknown>)) {
        if (key === variableName) {
            return child;
        }

        const found = searchValue(variableName, child, visited);
        if (found !== undefined) {
            return found;
        }
    }

    return undefined;
}
