// Copyright (c) 2026 dotandev
// SPDX-License-Identifier: Apache-2.0
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

import { NextResponse } from 'next/server';
import { execFile } from 'child_process';
import { promisify } from 'util';
import path from 'path';
import os from 'os';
import crypto from 'crypto';
import fs from 'fs/promises';

const execFileAsync = promisify(execFile);

// Exactly 64 hex characters — a Stellar/Soroban transaction hash.
const TX_HASH_PATTERN = /^[0-9a-fA-F]{64}$/;
const ALLOWED_NETWORKS = new Set(['testnet', 'mainnet', 'futurenet']);
const EXEC_TIMEOUT_MS = 30_000;

export async function POST(request: Request) {
  let svgPath: string | null = null;

  try {
    const body = await request.json();
    const txHash = typeof body?.txHash === 'string' ? body.txHash.trim() : '';
    const network = typeof body?.network === 'string' ? body.network : 'testnet';

    if (!TX_HASH_PATTERN.test(txHash)) {
      return NextResponse.json(
        {
          success: false,
          error: 'Invalid transaction hash',
          stderr: 'Transaction hash must be exactly 64 hexadecimal characters.',
        },
        { status: 400 },
      );
    }

    if (!ALLOWED_NETWORKS.has(network)) {
      return NextResponse.json({ success: false, error: 'Invalid network' }, { status: 400 });
    }

    const erstBinary = path.join(process.cwd(), '..', 'erst');
    svgPath = path.join(os.tmpdir(), `erst-callgraph-${crypto.randomUUID()}.svg`);

    // execFile with an argument array (no shell) — txHash/network never pass
    // through a shell, so they can't be used to inject extra commands/flags.
    const args = ['debug', txHash, '--network', network, '--export-svg', svgPath];

    try {
      const { stdout, stderr } = await execFileAsync(erstBinary, args, {
        timeout: EXEC_TIMEOUT_MS,
        maxBuffer: 10 * 1024 * 1024,
      });

      let parsedOutput: unknown;
      try {
        parsedOutput = JSON.parse(stdout);
      } catch {
        parsedOutput = stdout;
      }

      // --export-svg only writes a file when the simulation produced
      // diagnostic events, so a missing file here is a normal outcome.
      let callGraphSvg: string | undefined;
      try {
        callGraphSvg = await fs.readFile(svgPath, 'utf-8');
      } catch {
        callGraphSvg = undefined;
      }

      return NextResponse.json({
        success: true,
        output: parsedOutput,
        logs: stderr,
        callGraphSvg,
      });
    } catch (execError: any) {
      return NextResponse.json(
        {
          success: false,
          error: execError?.killed ? 'Simulation timed out' : execError?.message || 'Execution failed',
          stderr: execError?.stderr || '',
          stdout: execError?.stdout || '',
        },
        { status: 500 },
      );
    }
  } catch (err: any) {
    return NextResponse.json({ error: err?.message || 'Malformed request' }, { status: 400 });
  } finally {
    if (svgPath) {
      await fs.rm(svgPath, { force: true }).catch(() => {});
    }
  }
}
