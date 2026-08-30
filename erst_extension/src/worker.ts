// Copyright (c) Hintents Authors.
// SPDX-License-Identifier: Apache-2.0

import { parentPort } from 'worker_threads';
import * as fs from 'fs';

if (parentPort) {
    const port = parentPort;
    port.on('message', (msg: any) => {
        if (msg.type === 'loadTrace') {
            try {
                const trace = JSON.parse(fs.readFileSync(msg.filePath, 'utf8'));
                port.postMessage({ type: 'traceLoaded', trace });
            } catch (err: any) {
                port.postMessage({ type: 'traceLoadError', error: err.message });
            }
        }
    });
}