// Copyright (c) Hintents Authors.
// SPDX-License-Identifier: Apache-2.0

import * as assert from 'assert';
import * as net from 'net';
import * as vscode from 'vscode';
import type { DebugProtocol } from '@vscode/debugprotocol';
import {
    createMessageConnection,
    StreamMessageReader,
    StreamMessageWriter,
} from 'vscode-jsonrpc/node';
import type { Trace } from '../../erstClient';
import { ERSTDebugSession } from '../../dap/adapter';

/**
 * Starts a mock ERST simulator that speaks JSON-RPC over a TCP socket,
 * matching the protocol used by `ERSTClient`. It answers the
 * `DebugTransaction` and `GetTrace` requests without a real simulator.
 */
function startMockSimulator(): Promise<{
    port: number;
    close: () => Promise<void>;
}> {
    const trace: Trace = {
        transaction_hash: 'mock-tx',
        start_time: '2024-01-01T00:00:00Z',
        states: [
            {
                step: 1,
                timestamp: '2024-01-01T00:00:00.000Z',
                operation: 'invoke',
                function: 'increment',
                cpu_delta: 10,
                memory_delta: 5,
            },
            {
                step: 2,
                timestamp: '2024-01-01T00:00:00.100Z',
                operation: 'store',
                cpu_delta: 5,
                memory_delta: 2,
            },
        ],
    };

    return new Promise((resolve, reject) => {
        const sockets = new Set<net.Socket>();
        const server = net.createServer((socket) => {
            sockets.add(socket);
            socket.on('close', () => sockets.delete(socket));
            const connection = createMessageConnection(
                new StreamMessageReader(socket),
                new StreamMessageWriter(socket)
            );
            connection.onRequest('DebugTransaction', () => ({ ok: true }));
            connection.onRequest('GetTrace', () => trace);
            connection.listen();
        });
        server.on('error', reject);
        server.listen(0, '127.0.0.1', () => {
            const address = server.address() as net.AddressInfo;
            resolve({
                port: address.port,
                close: () =>
                    new Promise<void>((res) => {
                        for (const socket of sockets) {
                            socket.destroy();
                        }
                        sockets.clear();
                        server.close(() => res());
                    }),
            });
        });
    });
}

suite('ERST Extension', () => {
    const EXTENSION_ID = 'hintents.erst-vscode';

    test('Extension is present', () => {
        const ext = vscode.extensions.getExtension(EXTENSION_ID);
        assert.ok(ext, 'Extension should be registered');
    });

    test('Extension activates without error', async () => {
        const ext = vscode.extensions.getExtension(EXTENSION_ID);
        assert.ok(ext, 'Extension must be present to activate');
        await ext!.activate();
        assert.ok(ext!.isActive, 'Extension should be active after activation');
    });

    test('Commands are registered', async () => {
        const commands = await vscode.commands.getCommands(true);
        const expectedCommands = [
            'erst.triggerDebug',
            'erst.setTraceSearchQuery',
            'erst.exportTraceTree',
            'erst.selectTraceStep',
            'erst.showXdr',
            'erst.showStateDiff',
            'erst.nextTraceStep',
            'erst.prevTraceStep',
        ];
        for (const cmd of expectedCommands) {
            assert.ok(commands.includes(cmd), `${cmd} should be registered`);
        }
    });

    test('ERST trace view is registered', () => {
        const ext = vscode.extensions.getExtension(EXTENSION_ID);
        assert.ok(ext, 'Extension must be present');
        const contributes = ext!.packageJSON.contributes;
        assert.ok(contributes, 'Extension should have contributes section');
        const views = contributes.views;
        assert.ok(views, 'Extension should contribute views');
        assert.ok(views['erst-explorer'], 'Extension should contribute views for erst-explorer container');
        const erstViews = views['erst-explorer'];
        assert.ok(
            erstViews.some((v: { id: string }) => v.id === 'erst-traces'),
            'erst-traces view should be registered in erst-explorer'
        );
    });

    test('ERST debug session drives the DAP protocol against a mocked simulator', async () => {
        const mock = await startMockSimulator();

        try {
            // Instantiate the real inline debug adapter in the test runner and
            // drive it through the Debug Adapter Protocol, while its ERSTClient
            // talks to the mocked simulator over a real TCP JSON-RPC connection.
            const session = new ERSTDebugSession();
            const responses: DebugProtocol.Response[] = [];
            const events: DebugProtocol.Event[] = [];
            const subscription = session.onDidSendMessage((message: any) => {
                if (message.request_seq !== undefined) {
                    responses.push(message as DebugProtocol.Response);
                } else {
                    events.push(message as DebugProtocol.Event);
                }
            });

            // Sends a DAP request and resolves with its matching response body.
            let seqCounter = 1;
            const send = (
                command: string,
                args?: unknown
            ): Promise<DebugProtocol.Response['body']> =>
                new Promise((resolve, reject) => {
                    const requestSeq = seqCounter++;
                    const timer = setTimeout(() => {
                        reject(new Error(`Timed out waiting for response: ${command}`));
                    }, 15000);
                    session.handleMessage({
                        type: 'request',
                        seq: requestSeq,
                        command,
                        arguments: args,
                    } as unknown as DebugProtocol.Request);

                    const check = (): void => {
                        const response = responses.find(
                            (r) => r.request_seq === requestSeq
                        );
                        if (response) {
                            clearTimeout(timer);
                            if (response.success) {
                                resolve(response.body);
                            } else {
                                reject(new Error(JSON.stringify(response.body?.error)));
                            }
                            return;
                        }
                        setTimeout(check, 25);
                    };
                    check();
                });

            try {
                // initialize -> advertises adapter capabilities.
                const init = await send('initialize', {
                    clientID: 'test',
                    columnsStartAt1: true,
                    linesStartAt1: true,
                });
                assert.ok(
                    (init as DebugProtocol.InitializeResponse['body'])!.supportsConfigurationDoneRequest,
                    'initialize should advertise configurationDone support'
                );

                // launch -> connects to the mocked simulator.
                await send('launch', {
                    host: '127.0.0.1',
                    port: mock.port,
                    transactionHash: 'mock-tx',
                });

                // configurationDone -> loads the trace and pauses at first step.
                await send('configurationDone', {});
                assert.ok(
                    events.some(
                        (e) =>
                            (e as DebugProtocol.StoppedEvent).event === 'stopped'
                    ),
                    'configurationDone should emit a stopped event at the first step'
                );

                // evaluate "step" -> exposes the current trace step.
                const evalBody = await send('evaluate', {
                    expression: 'step',
                    frameId: 0,
                });
                const stepResult = (evalBody as DebugProtocol.EvaluateResponse['body']).result;
                assert.ok(
                    stepResult,
                    'evaluate "step" should return a trace step result'
                );
                const step = JSON.parse(stepResult);
                assert.strictEqual(
                    step.operation,
                    'invoke',
                    'evaluate "step" should expose the first trace step'
                );

                // stackTrace -> the call stack reflects the current operation.
                const framesBody = await send('stackTrace', { threadId: 1 });
                const frames = (
                    framesBody as DebugProtocol.StackTraceResponse['body']
                ).stackFrames;
                assert.ok(
                    frames.length >= 1,
                    'stackTrace should return at least one frame'
                );
                assert.ok(
                    frames[0].name.includes('invoke'),
                    'top frame should be the current operation'
                );

                // threads -> the single simulation thread.
                const threadsBody = await send('threads');
                assert.strictEqual(
                    (
                        threadsBody as DebugProtocol.ThreadsResponse['body']
                    ).threads.length,
                    1,
                    'threads should return exactly one thread'
                );

                // disconnect -> clean shutdown.
                await send('disconnect', {});
            } finally {
                subscription.dispose();
            }
        } finally {
            await mock.close();
        }
    });
});
