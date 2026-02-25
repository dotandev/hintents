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
            return Promise.resolve(
                this.currentTrace.states.map(step => new TraceItem(step))
            );
        }
    }
}

export class TraceItem extends vscode.TreeItem {
    constructor(
        public readonly step: TraceStep
    ) {
        const isStateUpdate = step.operation === 'StateUpdate' || step.operation === 'LedgerState';

        super(
            `${step.step}: ${step.operation}${step.function ? ` (${step.function})` : ''}`,
            isStateUpdate ? vscode.TreeItemCollapsibleState.Collapsed : vscode.TreeItemCollapsibleState.None
        );

        this.tooltip = `${this.label}`;
        this.description = step.error ? `Error: ${step.error}` : '';
        this.contextValue = 'traceStep';

        if (step.error) {
            this.iconPath = new (vscode.ThemeIcon as any)('error', new (vscode.ThemeColor as any)('errorForeground'));
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