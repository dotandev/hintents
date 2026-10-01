// Copyright (c) Hintents Authors.
// SPDX-License-Identifier: Apache-2.0

import * as net from 'net';

export class DAPServer {
    private server: net.Server;

    constructor(connectionHandler?: (socket: net.Socket) => void) {
        this.server = net.createServer(connectionHandler);
    }

    /**
     * Starts the DAP server with automatic port conflict resolution.
     * Detects EADDRINUSE and increments the port until an available one is found.
     * Notifies the IDE of the newly selected port via process.send.
     */
    public listen(startingPort: number = 8080, host: string = '127.0.0.1'): Promise<number> {
        return new Promise((resolve, reject) => {
            let currentPort = startingPort;

            this.server.on('error', (err: NodeJS.ErrnoException) => {
                if (err.code === 'EADDRINUSE') {
                    console.log(`[DAP] Port ${currentPort} is in use, trying ${currentPort + 1}...`);
                    currentPort++;
                    this.server.listen(currentPort, host);
                } else {
                    console.error(`[DAP] Server error: ${err.message}`);
                    reject(err);
                }
            });

            this.server.on('listening', () => {
                console.log(`[DAP] Server successfully started on port ${currentPort}`);
                
                // Notify IDE of the newly selected port
                if (process.send) {
                    process.send({ type: 'dap-port-selected', port: currentPort });
                }
                
                resolve(currentPort);
            });

            this.server.listen(currentPort, host);
        });
    }

    public close(): Promise<void> {
        return new Promise((resolve, reject) => {
            this.server.close((err) => {
                if (err) reject(err);
                else resolve();
            });
        });
    }
}
