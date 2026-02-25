// Copyright (c) Hintents Authors.
// SPDX-License-Identifier: Apache-2.0

import * as vscode from 'vscode';
import { Trace, TraceStep } from './erstClient';

export class TraceTreeDataProvider implements vscode.TreeDataProvider<vscode.TreeItem> {
    private _onDidChangeTreeData: vscode.EventEmitter<vscode.TreeItem | undefined | null> = new vscode.EventEmitter<vscode.TreeItem | undefined | null>();
    readonly onDidChangeTreeData: vscode.Event<vscode.TreeItem | undefined | null> = this._onDidChangeTreeData.event;

    private currentTrace: Trace | undefined;

    constructor() { }

    refresh(trace: Trace): void {
        this.currentTrace = trace;
        this._onDidChangeTreeData.fire(undefined);
    }

    getTreeItem(element: vscode.TreeItem): vscode.TreeItem {
        return element;
    }

    getChildren(element?: vscode.TreeItem): Thenable<vscode.TreeItem[]> {
        if (!this.currentTrace) {
            return Promise.resolve([]);
        }

        if (element instanceof TraceItem) {
            const step = element.step;
            const isStateUpdate = step.operation === 'StateUpdate' || step.operation === 'LedgerState';

            if (isStateUpdate && step.host_state) {
                const children: vscode.TreeItem[] = [];

                if (step.host_state.before) {
                    children.push(new StateDetailItem('Before State', step.host_state.before));
                }

                if (step.host_state.after) {
                    children.push(new StateDetailItem('After State', step.host_state.after));
                }

                if (step.host_state.before && step.host_state.after) {
                    children.push(new StateDiffItem(step.host_state.before, step.host_state.after));
                }

                return Promise.resolve(children);
            }
            return Promise.resolve([]);
        } else {
            const states = this.currentTrace.states;
            return Promise.resolve(
                states.map((step, idx) => new TraceItem(step, idx > 0 ? states[idx - 1] : undefined))
            );
        }
    }
}

export class TraceItem extends vscode.TreeItem {
    public isCrossContractBoundary: boolean;

    constructor(
        public readonly step: TraceStep,
        previousStep?: TraceStep
    ) {
        const isStateUpdate = step.operation === 'StateUpdate' || step.operation === 'LedgerState';

        super(
            `${step.step}: ${step.operation}${step.function ? ` (${step.function})` : ''}`,
            isStateUpdate ? vscode.TreeItemCollapsibleState.Collapsed : vscode.TreeItemCollapsibleState.None
        );

        this.isCrossContractBoundary = isCrossContractTransition(previousStep, step);

        // Build budget metrics display
        const budgetParts: string[] = [];
        if (step.cpu_delta !== undefined && step.cpu_delta > 0) {
            budgetParts.push(`CPU: ${this.formatNumber(step.cpu_delta)}`);
        }
        if (step.memory_delta !== undefined && step.memory_delta > 0) {
            budgetParts.push(`Mem: ${this.formatBytes(step.memory_delta)}`);
        }
        const budgetInfo = budgetParts.length > 0 ? ` [${budgetParts.join(', ')}]` : '';

        this.tooltip = `${this.label}${budgetInfo}`;
        this.description = step.error
            ? `Error: ${step.error}`
            : this.isCrossContractBoundary
                ? `[boundary] ${previousStep?.contract_id} -> ${step.contract_id}${budgetInfo}`
                : budgetInfo;
        this.contextValue = this.isCrossContractBoundary ? 'traceStepBoundary' : 'traceStep';

        if (step.error) {
            this.iconPath = new (vscode.ThemeIcon as any)('error', new (vscode.ThemeColor as any)('errorForeground'));
        } else if (this.isCrossContractBoundary) {
            this.iconPath = new (vscode.ThemeIcon as any)('git-compare', new (vscode.ThemeColor as any)('editorWarning.foreground'));
        } else if (isStateUpdate) {
            this.iconPath = new (vscode.ThemeIcon as any)('database', new (vscode.ThemeColor as any)('symbolIcon.fieldForeground'));
        } else {
            this.iconPath = new (vscode.ThemeIcon as any)('pass', new (vscode.ThemeColor as any)('debugIcon.startForeground'));
        }

        if (!isStateUpdate) {
            this.command = {
                command: 'erst.selectTraceStep',
                title: 'Select Trace Step',
                arguments: [this]
            };
        }
    }

    private formatNumber(num: number): string {
        if (num >= 1000000) {
            return `${(num / 1000000).toFixed(2)}M`;
        } else if (num >= 1000) {
            return `${(num / 1000).toFixed(2)}K`;
        }
        return num.toString();
    }

    private formatBytes(bytes: number): string {
        if (bytes >= 1048576) {
            return `${(bytes / 1048576).toFixed(2)}MB`;
        } else if (bytes >= 1024) {
            return `${(bytes / 1024).toFixed(2)}KB`;
        }
        return `${bytes}B`;
    }
}

// isCrossContractTransition returns true when two consecutive steps belong to different contracts.
function isCrossContractTransition(prev: TraceStep | undefined, current: TraceStep): boolean {
    if (!prev || !prev.contract_id || !current.contract_id) {
        return false;
    }
    return prev.contract_id !== current.contract_id;
}

export class StateDetailItem extends vscode.TreeItem {
    constructor(label: string, public readonly xdr: string) {
        super(label, vscode.TreeItemCollapsibleState.None);
        this.description = xdr.length > 30 ? xdr.substring(0, 30) + '...' : xdr;
        this.tooltip = `XDR: ${xdr}`;
        this.iconPath = new (vscode.ThemeIcon as any)('code');
        this.contextValue = 'stateDetail';
        this.command = {
            command: 'erst.showXdr',
            title: 'Show XDR',
            arguments: [xdr]
        };
    }
}

export class StateDiffItem extends vscode.TreeItem {
    constructor(public readonly before: string, public readonly after: string) {
        super('Visual Diff', vscode.TreeItemCollapsibleState.None);
        this.description = 'Compare states';
        this.tooltip = 'Show visual diff between before and after states';
        this.iconPath = new (vscode.ThemeIcon as any)('diff');
        this.contextValue = 'stateDiff';
        this.command = {
            command: 'erst.showStateDiff',
            title: 'Show State Diff',
            arguments: [before, after]
        };
    }
}
