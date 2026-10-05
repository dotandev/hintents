// Copyright (c) Hintents Authors.
// SPDX-License-Identifier: Apache-2.0

import * as rpc from 'vscode-jsonrpc/node';
import * as net from 'net';
import { connectWithRetry, ConnectionRetryConfig } from './ipc/lifecycle';

export interface TraceStep {
    step: number;
    timestamp: string;
    operation: string;
    contract_id?: string;
    function?: string;
    arguments?: any[];
    return_value?: any;
    error?: string;
    host_state?: any;
    memory?: any;
    cpu_delta?: number;
    memory_delta?: number;
}

export interface Trace {
    transaction_hash: string;
    start_time: string;
    states: TraceStep[];
}

export class ERSTClient {
    private connection: rpc.MessageConnection | undefined;
    private retryConfig?: ConnectionRetryConfig;

    constructor(
        private host: string = '127.0.0.1',
        private port: number = 8080,
        retryConfig?: ConnectionRetryConfig
    ) {
        this.retryConfig = retryConfig;
    }

    /**
     * Connects to the ERST simulator with retry logic.
     *
     * Uses exponential backoff to handle the race condition where
     * the DAP request arrives before the simulator has finished
     * binding its IPC socket.
     *
     * @throws Error if connection fails after all retries
     */
    async connect(): Promise<void> {
        const socket = await connectWithRetry(this.host, this.port, this.retryConfig);
        this.connection = rpc.createMessageConnection(
            new rpc.StreamMessageReader(socket),
            new rpc.StreamMessageWriter(socket)
        );
        this.connection.listen();
    }

    async debugTransaction(hash: string): Promise<any> {
        if (!this.connection) await this.connect();
        return this.connection!.sendRequest('DebugTransaction', { hash });
    }

    async getTrace(hash: string): Promise<Trace> {
        if (!this.connection) await this.connect();
        return this.connection!.sendRequest('GetTrace', { hash }) as Promise<Trace>;
    }

    dispose() {
        if (this.connection) {
            this.connection.dispose();
        }
    }
}
