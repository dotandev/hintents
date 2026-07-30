// Copyright (c) Hintents Authors.
// SPDX-License-Identifier: Apache-2.0

/**
 * ERST Debug Adapter Protocol (DAP) Implementation
 *
 * This module implements VS Code's Debug Adapter Protocol to support native
 * step-through debugging of Soroban smart contract execution traces. It wires
 * the VS Code debugger UI to ERST's IPC bridge, translating DAP requests into
 * simulator commands and mapping trace steps to stack frames.
 *
 * # Architecture
 *
 * ```
 * VS Code UI  <--DAP-->  ERSTDebugSession  <--JSON-RPC/TCP-->  ERST Simulator
 * ```
 *
 * The adapter is registered inline (runs in the extension host process) via
 * `vscode.debug.registerDebugAdapterDescriptorFactory`.
 *
 * # Supported DAP Requests
 *
 * | Request              | Description                                    |
 * |----------------------|------------------------------------------------|
 * | initialize           | Advertises adapter capabilities                |
 * | launch               | Connects to ERST simulator                     |
 * | configurationDone    | Starts debugging the transaction trace         |
 * | disconnect           | Disposes the IPC client                        |
 * | setBreakpoints       | Stores line/step breakpoints                   |
 * | setExceptionBreakpoints | Configures break-on-error behavior          |
 * | continue             | Resumes to next trace step                     |
 * | next                 | Steps to the next trace step                   |
 * | stepIn               | Same as next (trace-level stepping)            |
 * | stackTrace           | Returns current trace step as stack frame      |
 * | scopes               | Returns locals/arguments scopes                |
 * | variables            | Returns detailed step variables                |
 * | threads              | Returns single main thread                     |
 * | evaluate             | Evaluates expression in current trace context  |
 * | source               | Provides trace source content                  |
 * | terminate            | Terminates the debug session                   |
 */

import * as vscode from 'vscode';
import type { DebugProtocol } from '@vscode/debugprotocol';
import { ERSTClient, Trace, TraceStep } from '../erstClient';

// ---------------------------------------------------------------------------
// Variable Reference Registry
// ---------------------------------------------------------------------------

/**
 * Tracks mappings from numeric variable references to their underlying values
 * and paths within the trace data. This avoids hash collisions that would
 * occur with an encoded approach and supports arbitrary nesting depth.
 */
class VariableRefRegistry {
    private refCounter = 1; // 0 means "no children"
    private refs = new Map<number, { frameId: number; path: string; getter: () => any }>();

    /** Allocates a new variable reference for a getter function. */
    register(frameId: number, path: string, getter: () => any): number {
        const ref = this.refCounter++;
        this.refs.set(ref, { frameId, path, getter });
        return ref;
    }

    /** Retrieves the value for a previously-registered reference. */
    getValue(ref: number): any {
        return this.refs.get(ref)?.getter();
    }

    /** Clears all references (e.g. on a new trace). */
    reset(): void {
        this.refs.clear();
        this.refCounter = 1;
    }
}

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

/** The DAP debug type registered in package.json. */
export const ERST_DEBUG_TYPE = 'erst';

/** The virtual source scheme used for ERST trace sources. */
const ERST_SCHEME = 'erst-trace';

/** Thread ID for the single simulation thread. */
const MAIN_THREAD_ID = 1;

// ---------------------------------------------------------------------------
// ERSTDebugSession
// ---------------------------------------------------------------------------

/**
 * Inline debug adapter session that bridges VS Code's DAP to the ERST
 * simulator IPC layer. Each instance corresponds to one "launch" request.
 */
export class ERSTDebugSession implements vscode.DebugAdapter {
    private _onDidSendMessage = new vscode.EventEmitter<DebugProtocol.Message>();
    readonly onDidSendMessage: vscode.Event<DebugProtocol.Message> = this._onDidSendMessage.event;

    private client: ERSTClient;
    private trace: Trace | null = null;
    private currentStepIndex: number = 0;
    private config: any = {};
    private breakOnError: boolean = true;
    private stepBreakpoints: Set<number> = new Set();
    private isTerminated: boolean = false;
    private varRefs = new VariableRefRegistry();

    constructor() {
        this.client = new ERSTClient('127.0.0.1', 8080);
    }

    // ------------------------------------------------------------------
    // DAP Request Handler
    // ------------------------------------------------------------------

    handleMessage(message: DebugProtocol.ProtocolMessage): void {
        if (message.type !== 'request') return;

        const request = message as DebugProtocol.Request;

        try {
            switch (request.command) {
                case 'initialize':
                    this.handleInitialize(request);
                    break;
                case 'launch':
                    this.handleLaunch(request);
                    break;
                case 'attach':
                    this.handleAttach(request);
                    break;
                case 'disconnect':
                    this.handleDisconnect(request);
                    break;
                case 'terminate':
                    this.handleTerminate(request);
                    break;
                case 'configurationDone':
                    this.handleConfigurationDone(request);
                    break;
                case 'setBreakpoints':
                    this.handleSetBreakpoints(request);
                    break;
                case 'setExceptionBreakpoints':
                    this.handleSetExceptionBreakpoints(request);
                    break;
                case 'continue':
                    this.handleContinue(request);
                    break;
                case 'next':
                    this.handleNext(request);
                    break;
                case 'stepIn':
                    this.handleStepIn(request);
                    break;
                case 'stepOut':
                    this.handleStepOut(request);
                    break;
                case 'pause':
                    this.handlePause(request);
                    break;
                case 'stackTrace':
                    this.handleStackTrace(request);
                    break;
                case 'scopes':
                    this.handleScopes(request);
                    break;
                case 'variables':
                    this.handleVariables(request);
                    break;
                case 'source':
                    this.handleSource(request);
                    break;
                case 'threads':
                    this.handleThreads(request);
                    break;
                case 'evaluate':
                    this.handleEvaluate(request);
                    break;
                case 'exceptionInfo':
                    this.handleExceptionInfo(request);
                    break;
                default:
                    this.sendErrorResponse(request, {
                        id: 2000,
                        format: `Unknown command: {_command}`,
                        variables: { _command: request.command },
                    });
                    break;
            }
        } catch (err: any) {
            this.sendErrorResponse(request, {
                id: 9999,
                format: `Internal error: {_message}`,
                variables: { _message: err.message ?? String(err) },
                showUser: true,
            });
        }
    }

    // ------------------------------------------------------------------
    // Lifecycle Handlers
    // ------------------------------------------------------------------

    /**
     * Handles the 'initialize' request. Advertises the adapter's capabilities.
     */
    private handleInitialize(request: DebugProtocol.InitializeRequest): void {
        const response: DebugProtocol.InitializeResponse = {
            type: 'response',
            request_seq: request.seq,
            success: true,
            command: 'initialize',
            body: {
                supportsConfigurationDoneRequest: true,
                supportsSetVariable: false,
                supportsConditionalBreakpoints: false,
                supportsHitConditionalBreakpoints: false,
                supportsFunctionBreakpoints: true,
                supportsStepBack: true,
                supportsRestartFrame: false,
                supportsExceptionOptions: true,
                supportsExceptionInfoRequest: true,
                supportsEvaluateForHovers: true,
                supportsCompletionsRequest: false,
                supportsDelayedStackTraceLoading: true,
                supportsLogPoints: false,
                supportsTerminateRequest: true,
                supportsTerminateDebuggee: true,
                supportsRestartRequest: false,
                supportsValueFormattingOptions: false,
                supportsClipboardContext: false,
                supportsSteppingGranularity: false,
                supportsInstructionBreakpoints: false,
                supportsLoadedSourcesRequest: false,
                supportsReadMemoryRequest: false,
                supportsDisassembleRequest: false,
                exceptionBreakpointFilters: [
                    {
                        filter: 'all',
                        label: 'Runtime Errors',
                        description:
                            'Breaks when a trace step contains an error field',
                        default: true,
                    },
                    {
                        filter: 'uncaught',
                        label: 'Uncaught Errors',
                        description:
                            'Breaks only on unhandled errors in the trace',
                        default: false,
                    },
                ],
            },
        };
        this.sendMessage(response);
    }

    /**
     * Handles the 'launch' request. Connects to the ERST simulator.
     */
    private handleLaunch(request: DebugProtocol.LaunchRequest): void {
        this.config = request.arguments || {};
        const host = this.config.host || '127.0.0.1';
        const port = this.config.port ?? 8080;

        // Re-create client with configured address
        this.client = new ERSTClient(host, port);

        this.client
            .connect()
            .then(() => {
                this.sendMessage({
                    type: 'response',
                    request_seq: request.seq,
                    success: true,
                    command: 'launch',
                } as DebugProtocol.LaunchResponse);
            })
            .catch((err: Error) => {
                this.sendErrorResponse(request, {
                    id: 1001,
                    format:
                        'Failed to connect to ERST simulator at {host}:{port}: {_message}',
                    variables: {
                        host,
                        port: String(port),
                        _message: err.message,
                    },
                    showUser: true,
                });
            });
    }

    /**
     * Handles the 'attach' request. Attaches to an already-running simulation.
     */
    private handleAttach(request: DebugProtocol.AttachRequest): void {
        this.config = request.arguments || {};
        const host = this.config.host || '127.0.0.1';
        const port = this.config.port ?? 8080;

        this.client = new ERSTClient(host, port);

        this.client
            .connect()
            .then(() => {
                this.sendMessage({
                    type: 'response',
                    request_seq: request.seq,
                    success: true,
                    command: 'attach',
                } as DebugProtocol.AttachResponse);
            })
            .catch((err: Error) => {
                this.sendErrorResponse(request, {
                    id: 1011,
                    format:
                        'Failed to attach to ERST simulator at {host}:{port}: {_message}',
                    variables: {
                        host,
                        port: String(port),
                        _message: err.message,
                    },
                    showUser: true,
                });
            });
    }

    /**
     * Handles the 'disconnect' request. Cleanly shuts down the session.
     */
    private handleDisconnect(request: DebugProtocol.DisconnectRequest): void {
        this.cleanup();
        this.sendMessage({
            type: 'response',
            request_seq: request.seq,
            success: true,
            command: 'disconnect',
        } as DebugProtocol.DisconnectResponse);
    }

    /**
     * Handles the 'terminate' request.
     */
    private handleTerminate(
        request: DebugProtocol.TerminateRequest
    ): void {
        this.cleanup();
        this.sendMessage({
            type: 'response',
            request_seq: request.seq,
            success: true,
            command: 'terminate',
        } as DebugProtocol.TerminateResponse);
    }

    /**
     * Handles the 'configurationDone' request. Starts debugging the
     * transaction and emits a StoppedEvent at the first trace step.
     */
    private handleConfigurationDone(
        request: DebugProtocol.ConfigurationDoneRequest
    ): void {
        const txHash: string =
            this.config.transactionHash || this.config.txHash || '';

        if (!txHash) {
            this.sendErrorResponse(request, {
                id: 1003,
                format:
                    'No transaction hash specified. Set "transactionHash" in your launch configuration.',
                showUser: true,
            });
            return;
        }

        this.client
            .debugTransaction(txHash)
            .then(() => this.client.getTrace(txHash))
            .then((trace: Trace) => {
                this.trace = trace;
                this.currentStepIndex = 0;
                this.isTerminated = false;
                this.varRefs.reset();

                // Log trace info to debug console
                this.sendEventMessage(
                    'stdout',
                    `ERST: Loaded trace "${txHash}" with ${trace.states.length} step(s)\n`
                );

                this.sendMessage({
                    type: 'response',
                    request_seq: request.seq,
                    success: true,
                    command: 'configurationDone',
                } as DebugProtocol.ConfigurationDoneResponse);

                // Pause at the first step
                this.sendEventMessage(
                    'stdout',
                    `ERST: Stopped at step ${this.currentStepIndex + 1}: ${this.getCurrentStep()?.operation ?? 'start'}\n`
                );

                this.sendStoppedEvent('entry');
            })
            .catch((err: Error) => {
                this.sendErrorResponse(request, {
                    id: 1002,
                    format:
                        'Failed to debug transaction "{_hash}": {_message}',
                    variables: {
                        _hash: txHash,
                        _message: err.message,
                    },
                    showUser: true,
                });
            });
    }

    // ------------------------------------------------------------------
    // Breakpoint Handlers
    // ------------------------------------------------------------------

    private handleSetBreakpoints(
        request: DebugProtocol.SetBreakpointsRequest
    ): void {
        const args = request.arguments;
        const bps: DebugProtocol.Breakpoint[] = (args.breakpoints || []).map(
            (bp) => {
                // Store breakpoint by line number (trace step index)
                this.stepBreakpoints.add(bp.line);
                return {
                    id: bp.line,
                    verified: true,
                    line: bp.line,
                } as DebugProtocol.Breakpoint;
            }
        );

        this.sendMessage({
            type: 'response',
            request_seq: request.seq,
            success: true,
            command: 'setBreakpoints',
            body: { breakpoints: bps },
        } as DebugProtocol.SetBreakpointsResponse);
    }

    private handleSetExceptionBreakpoints(
        request: DebugProtocol.SetExceptionBreakpointsRequest
    ): void {
        const args = request.arguments;
        this.breakOnError =
            args.filters.includes('all') ||
            args.filters.includes('uncaught');

        this.sendMessage({
            type: 'response',
            request_seq: request.seq,
            success: true,
            command: 'setExceptionBreakpoints',
        } as DebugProtocol.SetExceptionBreakpointsResponse);
    }

    // ------------------------------------------------------------------
    // Stepping Handlers
    // ------------------------------------------------------------------

    private handleContinue(
        request: DebugProtocol.ContinueRequest
    ): void {
        if (!this.trace || this.currentStepIndex >= this.trace.states.length - 1) {
            this.sendMessage({
                type: 'response',
                request_seq: request.seq,
                success: true,
                command: 'continue',
            } as DebugProtocol.ContinueResponse);
            this.sendTerminatedEvent();
            return;
        }

        // Advance to next step
        this.currentStepIndex++;

        // Check if we hit a breakpoint
        const hitBreakpoint = this.stepBreakpoints.has(this.currentStepIndex);
        const hasError = this.trace.states[this.currentStepIndex]?.error != null;
        const shouldPause = hitBreakpoint || (hasError && this.breakOnError);

        this.sendMessage({
            type: 'response',
            request_seq: request.seq,
            success: true,
            command: 'continue',
            body: { allThreadsContinued: true },
        } as DebugProtocol.ContinueResponse);

        if (shouldPause) {
            const reason = hitBreakpoint ? 'breakpoint' : 'exception';
            this.sendStoppedEvent(reason);
        } else if (this.currentStepIndex >= this.trace.states.length - 1) {
            this.sendTerminatedEvent();
        }
        // If not at breakpoint and not at end, we'd continue running
        // For trace debugging, we stop at each step by default
    }

    private handleNext(request: DebugProtocol.NextRequest): void {
        if (!this.trace || this.currentStepIndex >= this.trace.states.length - 1) {
            this.sendMessage({
                type: 'response',
                request_seq: request.seq,
                success: true,
                command: 'next',
            } as DebugProtocol.NextResponse);
            this.sendTerminatedEvent();
            return;
        }

        this.currentStepIndex++;
        const step = this.trace.states[this.currentStepIndex];

        // Log step info
        this.sendEventMessage(
            'stdout',
            `ERST: Step ${this.currentStepIndex + 1}/${this.trace.states.length}: ${step.operation}${step.function ? ` (${step.function})` : ''}\n`
        );

        this.sendMessage({
            type: 'response',
            request_seq: request.seq,
            success: true,
            command: 'next',
        } as DebugProtocol.NextResponse);

        // Check for errors
        if (step.error && this.breakOnError) {
            this.sendEventMessage(
                'stderr',
                `ERST: Error at step ${this.currentStepIndex + 1}: ${step.error}\n`
            );
            this.sendStoppedEvent('exception');
            return;
        }

        if (this.currentStepIndex >= this.trace.states.length - 1) {
            this.sendTerminatedEvent();
            return;
        }

        this.sendStoppedEvent('step');
    }

    private handleStepIn(request: DebugProtocol.StepInRequest): void {
        // For trace-based debugging, step-in is equivalent to step-over
        // since each trace step is the minimum granularity
        this.handleNext(
            request as unknown as DebugProtocol.NextRequest
        );
    }

    private handleStepOut(request: DebugProtocol.StepOutRequest): void {
        // For trace debugging, step-out advances to the end of the trace
        // (or to the next cross-contract boundary)
        if (!this.trace) {
            this.sendMessage({
                type: 'response',
                request_seq: request.seq,
                success: true,
                command: 'stepOut',
            } as DebugProtocol.StepOutResponse);
            this.sendTerminatedEvent();
            return;
        }

        // Advance to the last step
        this.currentStepIndex = this.trace.states.length - 1;

        this.sendMessage({
            type: 'response',
            request_seq: request.seq,
            success: true,
            command: 'stepOut',
        } as DebugProtocol.StepOutResponse);

        this.sendStoppedEvent('step');
    }

    private handlePause(request: DebugProtocol.PauseRequest): void {
        this.sendMessage({
            type: 'response',
            request_seq: request.seq,
            success: true,
            command: 'pause',
        } as DebugProtocol.PauseResponse);
        this.sendStoppedEvent('pause');
    }

    // ------------------------------------------------------------------
    // Introspection Handlers
    // ------------------------------------------------------------------

    private handleStackTrace(
        request: DebugProtocol.StackTraceRequest
    ): void {
        if (!this.trace) {
            this.sendMessage({
                type: 'response',
                request_seq: request.seq,
                success: true,
                command: 'stackTrace',
                body: { stackFrames: [], totalFrames: 0 },
            } as DebugProtocol.StackTraceResponse);
            return;
        }

        const args = request.arguments;
        const startFrame = args.startFrame ?? 0;
        const levels = args.levels ?? 20;
        const step = this.getCurrentStep();

        if (!step) {
            this.sendMessage({
                type: 'response',
                request_seq: request.seq,
                success: true,
                command: 'stackTrace',
                body: { stackFrames: [], totalFrames: 0 },
            } as DebugProtocol.StackTraceResponse);
            return;
        }

        // Build stack frames for the current step
        // The "call stack" shows the trace step with contextual info
        const frames: DebugProtocol.StackFrame[] = [];
        const idx = this.currentStepIndex + 1; // 1-based for display
        const txHash = this.trace.transaction_hash;

        // --- Top frame: the current operation being executed ---
        const frameName = `${step.operation}${step.function ? `: ${step.function}` : ''}`;
        frames.push({
            id: this.currentStepIndex,
            name: frameName,
            source: {
                name: `${txHash.slice(0, 8)}.trace`,
                path: `${ERST_SCHEME}://${txHash}/step/${this.currentStepIndex}`,
                sourceReference: 0,
            },
            line: idx,
            column: 1,
            presentationHint: 'normal',
        });

        // --- Sub-frame for cross-contract boundary info ---
        if (step.contract_id) {
            const contractLabel = `contract: ${step.contract_id.slice(0, 12)}...`;
            frames.push({
                id: this.currentStepIndex + 10000,
                name: contractLabel,
                source: {
                    name: `${step.contract_id.slice(0, 8)}.wasm`,
                    path: `${ERST_SCHEME}://contract/${step.contract_id}`,
                    sourceReference: 0,
                },
                line: idx,
                column: 1,
                presentationHint: 'subtle',
            });
        }

        return this.sendMessage({
            type: 'response',
            request_seq: request.seq,
            success: true,
            command: 'stackTrace',
            body: {
                stackFrames: frames.slice(startFrame, startFrame + levels),
                totalFrames: frames.length,
            },
        } as DebugProtocol.StackTraceResponse);
    }

    private handleScopes(
        request: DebugProtocol.ScopesRequest
    ): void {
        const step = this.getCurrentStep();
        if (!step) {
            return this.sendMessage({
                type: 'response',
                request_seq: request.seq,
                success: true,
                command: 'scopes',
                body: { scopes: [] },
            } as DebugProtocol.ScopesResponse);
        }

        const frameId = request.arguments.frameId;
        const scopes: DebugProtocol.Scope[] = [];
        const fid = this.currentStepIndex;

        // Locals scope - contains step operation details
        scopes.push({
            name: 'Locals',
            variablesReference: this.varRefs.register(fid, 'locals', () => ({
                step: step.step,
                operation: step.operation,
                timestamp: step.timestamp,
                contract_id: step.contract_id,
                function: step.function,
                return_value: step.return_value,
                error: step.error,
            })),
            namedVariables: 7,
            indexedVariables: 0,
            expensive: false,
            presentationHint: 'locals',
        });

        // Arguments scope - contains function call arguments
        if (step.arguments !== undefined) {
            scopes.push({
                name: 'Arguments',
                variablesReference: this.varRefs.register(fid, 'arguments', () => step.arguments),
                namedVariables: 1,
                indexedVariables: 0,
                expensive: false,
                presentationHint: 'arguments',
            });
        }

        // Host State scope - contains ledger state before/after
        if (step.host_state !== undefined) {
            scopes.push({
                name: 'Host State',
                variablesReference: this.varRefs.register(fid, 'hostState', () => step.host_state),
                namedVariables: 3,
                indexedVariables: 0,
                expensive: true,
                presentationHint: 'registers',
            });
        }

        // Memory scope - contains memory snapshot
        if (step.memory !== undefined) {
            scopes.push({
                name: 'Memory',
                variablesReference: this.varRefs.register(fid, 'memory', () => step.memory),
                namedVariables: 1,
                indexedVariables: 0,
                expensive: true,
                presentationHint: 'registers',
            });
        }

        // Budget scope - contains CPU/memory delta information
        scopes.push({
            name: 'Budget',
            variablesReference: this.varRefs.register(fid, 'budget', () => ({
                cpu_delta: step.cpu_delta ?? 0,
                memory_delta: step.memory_delta ?? 0,
            })),
            namedVariables: 2,
            indexedVariables: 0,
            expensive: false,
            presentationHint: 'registers',
        });

        this.sendMessage({
            type: 'response',
            request_seq: request.seq,
            success: true,
            command: 'scopes',
            body: { scopes },
        } as DebugProtocol.ScopesResponse);
    }

    private handleVariables(
        request: DebugProtocol.VariablesRequest
    ): void {
        const ref = request.arguments.variablesReference;
        const parentValue = this.varRefs.getValue(ref);

        const variables = this.getChildVariables(parentValue, ref);

        this.sendMessage({
            type: 'response',
            request_seq: request.seq,
            success: true,
            command: 'variables',
            body: { variables },
        } as DebugProtocol.VariablesResponse);
    }

    private handleSource(
        request: DebugProtocol.SourceRequest
    ): void {
        // For ERST, "source" returns a description of the trace at the
        // given position rather than actual source code (since the
        // execution is on WASM bytecode)
        const sourceRef = request.arguments.sourceReference;
        const content = this.getSourceContent(sourceRef);

        this.sendMessage({
            type: 'response',
            request_seq: request.seq,
            success: true,
            command: 'source',
            body: {
                content,
                mimeType: 'application/json',
            },
        } as DebugProtocol.SourceResponse);
    }

    private handleThreads(
        request: DebugProtocol.ThreadsRequest
    ): void {
        this.sendMessage({
            type: 'response',
            request_seq: request.seq,
            success: true,
            command: 'threads',
            body: {
                threads: [{ id: MAIN_THREAD_ID, name: 'Main Thread' }],
            },
        } as DebugProtocol.ThreadsResponse);
    }

    private handleEvaluate(
        request: DebugProtocol.EvaluateRequest
    ): void {
        const expr = request.arguments.expression;
        const step = this.getCurrentStep();

        if (!step) {
            return this.sendMessage({
                type: 'response',
                request_seq: request.seq,
                success: false,
                command: 'evaluate',
                body: {
                    result: 'No active trace step',
                    variablesReference: 0,
                },
            } as DebugProtocol.EvaluateResponse);
        }

        // Simple expression evaluator for the trace context
        let result: string | undefined;

        // Check for simple property access
        const trimmed = expr.trim();
        if (trimmed === 'this' || trimmed === 'step') {
            result = JSON.stringify(step, null, 2);
        } else if (trimmed.startsWith('step.')) {
            const keys = trimmed.slice(5).split('.');
            let value: any = step;
            for (const key of keys) {
                if (value != null && typeof value === 'object') {
                    value = (value as Record<string, any>)[key];
                } else {
                    value = undefined;
                    break;
                }
            }
            result = value !== undefined ? JSON.stringify(value, null, 2) : undefined;
        } else if (trimmed === 'trace') {
            result = JSON.stringify(this.trace, null, 2);
        } else if (trimmed === 'steps' || trimmed === 'states') {
            result = JSON.stringify(this.trace?.states, null, 2);
        } else if (!isNaN(Number(trimmed))) {
            const idx = parseInt(trimmed, 10);
            result = JSON.stringify(this.trace?.states[idx], null, 2);
        }

        if (result !== undefined) {
            this.sendMessage({
                type: 'response',
                request_seq: request.seq,
                success: true,
                command: 'evaluate',
                body: {
                    result,
                    variablesReference: 0,
                    type: 'object',
                    presentationHint: 'normal',
                },
            } as DebugProtocol.EvaluateResponse);
        } else {
            this.sendMessage({
                type: 'response',
                request_seq: request.seq,
                success: false,
                command: 'evaluate',
                body: {
                    result: `Unknown expression: ${trimmed}`,
                    variablesReference: 0,
                },
            } as DebugProtocol.EvaluateResponse);
        }
    }

    private handleExceptionInfo(
        request: DebugProtocol.ExceptionInfoRequest
    ): void {
        const step = this.getCurrentStep();
        if (step?.error) {
            this.sendMessage({
                type: 'response',
                request_seq: request.seq,
                success: true,
                command: 'exceptionInfo',
                body: {
                    exceptionId: 'RuntimeError',
                    description: step.error,
                    breakMode: 'always',
                    details: {
                        message: step.error,
                        stackTrace: `Step ${this.currentStepIndex + 1}: ${step.operation}`,
                    },
                },
            } as DebugProtocol.ExceptionInfoResponse);
        } else {
            this.sendMessage({
                type: 'response',
                request_seq: request.seq,
                success: true,
                command: 'exceptionInfo',
                body: {
                    exceptionId: 'ok',
                    breakMode: 'never',
                },
            } as DebugProtocol.ExceptionInfoResponse);
        }
    }

    // ------------------------------------------------------------------
    // Variable Helpers
    // ------------------------------------------------------------------

    // Individual scope variable getters are now defined inline as
    // closures registered with `VariableRefRegistry` in `handleScopes`.

    /**
     * Returns child variables for a nested object/array at the given reference.
     * Each child value that is itself an object/array gets a new variable reference
     * registered with the `varRefs` registry so it can be expanded further.
     */
    private getChildVariables(
        parentValue: any,
        _parentRef: number
    ): DebugProtocol.Variable[] {
        const variables: DebugProtocol.Variable[] = [];

        if (parentValue == null || typeof parentValue !== 'object') {
            return variables;
        }

        if (Array.isArray(parentValue)) {
            for (let i = 0; i < parentValue.length; i++) {
                const item = parentValue[i];
                const childRef =
                    item !== null && typeof item === 'object'
                        ? this.varRefs.register(this.currentStepIndex, `array[${i}]`, () => item)
                        : 0;
                variables.push({
                    name: `[${i}]`,
                    value: this.formatValue(item),
                    type: typeof item === 'object' && item !== null ? 'object' : typeof item,
                    variablesReference: childRef,
                });
            }
        } else {
            const entries = Object.entries(parentValue as Record<string, any>);
            for (const [key, val] of entries) {
                const childRef =
                    val !== null && typeof val === 'object'
                        ? this.varRefs.register(this.currentStepIndex, `obj.${key}`, () => val)
                        : 0;
                variables.push({
                    name: key,
                    value: this.formatValue(val),
                    type: typeof val === 'object' && val !== null ? 'object' : typeof val,
                    variablesReference: childRef,
                });
            }
        }

        return variables;
    }

    // Nested variable value resolution is handled by VariableRefRegistry

    // ------------------------------------------------------------------
    // Variable Reference Helpers
    // ------------------------------------------------------------------
    // Variable references are managed by `VariableRefRegistry`, which
    // maps numeric IDs to getter functions. This approach eliminates
    // hash collisions and supports arbitrary nesting depth.

    // ------------------------------------------------------------------
    // Utility Methods
    // ------------------------------------------------------------------

    /**
     * Returns the current trace step, or `undefined` if the session
     * hasn't loaded a trace yet.
     */
    private getCurrentStep(): TraceStep | undefined {
        return this.trace?.states[this.currentStepIndex];
    }

    /**
     * Formats a value for display in the debugger variables pane.
     */
    private formatValue(value: any): string {
        if (value === null) return 'null';
        if (value === undefined) return 'undefined';
        if (typeof value === 'string') {
            if (value.length > 80) return `"${value.slice(0, 77)}..."`;
            return `"${value}"`;
        }
        if (typeof value === 'number') return String(value);
        if (typeof value === 'boolean') return String(value);
        if (Array.isArray(value)) return `Array(${value.length})`;
        if (typeof value === 'object') {
            const keys = Object.keys(value);
            return `Object(${keys.length})`;
        }
        return String(value);
    }

    /**
     * Returns a source content string for a given source reference.
     */
    private getSourceContent(_sourceReference: number): string {
        if (!this.trace) return '';

        // Return the full trace as a formatted JSON source
        return JSON.stringify(
            {
                transaction_hash: this.trace.transaction_hash,
                total_steps: this.trace.states.length,
                current_step: this.currentStepIndex,
                step_detail: this.getCurrentStep(),
            },
            null,
            2
        );
    }

    /**
     * Cleans up resources when the debug session ends.
     */
    private cleanup(): void {
        if (!this.isTerminated) {
            this.isTerminated = true;
            try {
                this.client.dispose();
            } catch {
                // Ignore cleanup errors
            }
            this.trace = null;
            this.currentStepIndex = 0;
        }
    }

    // ------------------------------------------------------------------
    // Message Sending Helpers
    // ------------------------------------------------------------------

    private sendMessage(msg: DebugProtocol.Message): void {
        this._onDidSendMessage.fire(msg);
    }

    private sendErrorResponse(
        request: DebugProtocol.Request,
        error: {
            id: number;
            format: string;
            variables?: Record<string, string>;
            showUser?: boolean;
        }
    ): void {
        this.sendMessage({
            type: 'response',
            request_seq: request.seq,
            success: false,
            command: request.command,
            body: {
                error: {
                    id: error.id,
                    format: error.format,
                    variables: error.variables,
                    showUser: error.showUser ?? false,
                },
            },
        } as DebugProtocol.ErrorResponse);
    }

    private sendStoppedEvent(reason: string): void {
        const step = this.getCurrentStep();
        const description = step?.error
            ? `Error: ${step.error}`
            : `Step ${this.currentStepIndex + 1}: ${step?.operation ?? 'unknown'}`;

        this.sendMessage({
            type: 'event',
            event: 'stopped',
            body: {
                reason,
                threadId: MAIN_THREAD_ID,
                allThreadsStopped: true,
                description,
                text: description,
            },
        } as DebugProtocol.StoppedEvent);
    }

    private sendContinuedEvent(): void {
        this.sendMessage({
            type: 'event',
            event: 'continued',
            body: {
                threadId: MAIN_THREAD_ID,
                allThreadsContinued: true,
            },
        } as DebugProtocol.ContinuedEvent);
    }

    private sendTerminatedEvent(): void {
        this.cleanup();
        this.sendMessage({
            type: 'event',
            event: 'terminated',
            body: {
                restart: false,
            },
        } as DebugProtocol.TerminatedEvent);
    }

    private sendEventMessage(
        category: 'stdout' | 'stderr' | 'console',
        message: string
    ): void {
        this.sendMessage({
            type: 'event',
            event: 'output',
            body: {
                category,
                output: message,
            },
        } as DebugProtocol.OutputEvent);
    }
}

// ---------------------------------------------------------------------------
// DebugAdapterDescriptorFactory
// ---------------------------------------------------------------------------

/**
 * Factory that creates inline ERST debug adapter sessions when VS Code
 * launches a debug configuration of type "erst".
 *
 * Register this in your extension's activate() function:
 * ```ts
 * context.subscriptions.push(
 *   vscode.debug.registerDebugAdapterDescriptorFactory(
 *     ERST_DEBUG_TYPE,
 *     new ERSTDebugAdapterFactory()
 *   )
 * );
 * ```
 */
export class ERSTDebugAdapterFactory
    implements vscode.DebugAdapterDescriptorFactory
{
    createDebugAdapterDescriptor(
        _session: vscode.DebugSession
    ): vscode.ProviderResult<vscode.DebugAdapterDescriptor> {
        return new vscode.DebugInlineAdapter(new ERSTDebugSession());
    }
}

// ---------------------------------------------------------------------------
// DebugConfigurationProvider
// ---------------------------------------------------------------------------

/**
 * Provides ERST-specific debug configurations and resolves variables in
 * launch configurations before they are passed to the debug adapter.
 */
export class ERSTDebugConfigurationProvider
    implements vscode.DebugConfigurationProvider
{
    /**
     * Provides the initial debug configurations presented in the
     * launch.json dropdown.
     */
    provideDebugConfigurations(
        _folder?: vscode.WorkspaceFolder,
        _token?: vscode.CancellationToken
    ): vscode.ProviderResult<vscode.DebugConfiguration[]> {
        return [
            {
                type: ERST_DEBUG_TYPE,
                request: 'launch',
                name: 'ERST: Debug Transaction',
                transactionHash: '',
                host: '127.0.0.1',
                port: 8080,
            },
            {
                type: ERST_DEBUG_TYPE,
                request: 'attach',
                name: 'ERST: Attach to Simulator',
                host: '127.0.0.1',
                port: 8080,
            },
        ];
    }

    /**
     * Resolves or modifies a debug configuration before it's passed to
     * the debug adapter.
     */
    resolveDebugConfiguration(
        folder: vscode.WorkspaceFolder | undefined,
        debugConfiguration: vscode.DebugConfiguration,
        _token?: vscode.CancellationToken
    ): vscode.ProviderResult<vscode.DebugConfiguration> {
        // If the configuration is missing required fields, prompt the user
        if (
            !debugConfiguration.transactionHash &&
            debugConfiguration.request === 'launch'
        ) {
            return vscode.window
                .showInputBox({
                    prompt: 'Enter the transaction hash to debug',
                    placeHolder: 'e.g., a1b2c3d4e5f6...',
                    ignoreFocusOut: true,
                })
                .then((hash) => {
                    if (hash) {
                        debugConfiguration.transactionHash = hash;
                        return debugConfiguration;
                    }
                    // User cancelled - return undefined to abort
                    return undefined;
                });
        }

        return debugConfiguration;
    }
}
