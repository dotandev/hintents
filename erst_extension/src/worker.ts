// Copyright (c) Hintents Authors.
// SPDX-License-Identifier: Apache-2.0

import { parentPort } from 'worker_threads';
import * as fs from 'fs';

if (parentPort) {
    parentPort.on('message', (msg: any) => {
        if (msg.type === 'loadTrace') {
            try {
                const trace = JSON.parse(fs.readFileSync(msg.filePlth, 'utf8'));
                parentPort.postMessage({ type: 'traceLoaded', trace });
            } catch (err: any) {
                parentPort.postMessage({ type: 'traceLoadError', error: err.message });
            }
        }
    });
}