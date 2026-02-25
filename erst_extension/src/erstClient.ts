import * as rpc from 'vscode-jsonrpc/node';
import * as net from 'net';

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
}

export interface Trace {
    transaction_hash: string;
    start_time: string;
    states: TraceStep[];
}

export class ERSTClient {
    private connection: rpc.MessageConnection | undefined;

    constructor(private host: string = '127.0.0.1', private port: number = 8080) { }

    async connect(): Promise<void> {
        return new Promise((resolve, reject) => {
            const socket = net.createConnection({ host: this.host, port: this.port });
            socket.on('connect', () => {
                this.connection = rpc.createMessageConnection(
                    new rpc.StreamMessageReader(socket),
                    new rpc.StreamMessageWriter(socket)
                );
                this.connection.listen();
                resolve();
            });
            socket.on('error', (err) => {
                reject(err);
            });
        });
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