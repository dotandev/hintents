// Copyright (c) Hintents Authors.
// SPDX-License-Identifier: Apache-2.0

import * as path from 'path';
import Mocha from 'mocha';

export function run(): Promise<void> {
    const mocha = new Mocha({
        ui: 'tdd',
        color: true,
        timeout: 60_000,
    });

    // Load the single test suite file directly (no glob needed for one file).
    mocha.addFile(path.resolve(__dirname, 'extension.test.js'));

    return new Promise((resolve, reject) => {
        try {
            mocha.run((failures) => {
                if (failures > 0) {
                    reject(new Error(`${failures} test(s) failed.`));
                } else {
                    resolve();
                }
            });
        } catch (err) {
            reject(err);
        }
    });
}
