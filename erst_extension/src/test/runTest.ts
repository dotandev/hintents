// Copyright (c) Hintents Authors.
// SPDX-License-Identifier: Apache-2.0

import * as path from 'path';
import { runTests } from '@vscode/test-electron';

async function main(): Promise<void> {
    try {
        const extensionDevelopmentPath = path.resolve(__dirname, '../../');
        const extensionTestsPath = path.resolve(__dirname, './suite/index');

        await runTests({
            extensionDevelopmentPath,
            extensionTestsPath,
            launchArgs: [
                '--no-sandbox',
                '--disable-gpu',
                '--disable-software-rasterizer',
                '--disable-dev-shm-usage',
                '--user-data-dir',
                path.resolve(__dirname, '../../.vscode-test/user-data'),
            ],
        });
    } catch (err) {
        console.error('Integration tests failed:', err);
        process.exit(1);
    }
}

main();
