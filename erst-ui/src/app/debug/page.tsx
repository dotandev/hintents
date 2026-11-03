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

"use client";

import React, { useState } from 'react';
import Link from 'next/link';
import { ArrowLeft, CheckCircle2, XCircle } from 'lucide-react';
import styles from './page.module.css';
import Button from '@/components/ui/Button';
import Logo from '@/components/icons/Logo';
import ThemeToggle from '@/components/ThemeToggle';

type Network = 'testnet' | 'mainnet' | 'futurenet';

interface DebugResult {
  success: boolean;
  output?: unknown;
  logs?: string;
  callGraphSvg?: string;
  error?: string;
  stderr?: string;
  stdout?: string;
}

// SVG text content is generated server-side from simulated contract call
// data (function names, contract IDs), which an adversarial contract could
// influence. Rendering it as an <img> — rather than injecting it into the
// DOM — keeps any embedded markup inert: the browser treats the whole
// document as a static image and never executes scripts inside it.
function svgToImageSrc(svg: string): string {
  const encoded = encodeURIComponent(svg).replace(/'/g, '%27').replace(/"/g, '%22');
  return `data:image/svg+xml,${encoded}`;
}

export default function DebugPage() {
  const [txHash, setTxHash] = useState('');
  const [network, setNetwork] = useState<Network>('testnet');
  const [loading, setLoading] = useState(false);
  const [result, setResult] = useState<DebugResult | null>(null);

  const handleDebug = async () => {
    if (!txHash) return;

    // Client-side validation for transaction hash (64 hex characters).
    // The API route re-validates this server-side — this is just fast feedback.
    if (txHash.length !== 64 || !/^[0-9a-fA-F]+$/.test(txHash)) {
      setResult({
        success: false,
        error: 'Invalid Transaction Hash',
        stderr: 'Transaction hash must be exactly 64 hex characters long.',
      });
      return;
    }

    setLoading(true);
    setResult(null);
    try {
      const res = await fetch('/api/debug', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ txHash, network }),
      });
      const data = await res.json();
      setResult(data);
    } catch (err) {
      setResult({ success: false, error: 'Failed to execute request' });
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className={styles.page}>
      <nav className={styles.nav}>
        <Link href="/" className={styles.navBrand}>
          <span className={styles.navBrandIcon}>
            <Logo size={22} />
          </span>
          Erst. <span className={styles.navCrumb}>/ Debugger</span>
        </Link>
        <div className={styles.navActions}>
          <Link href="/" className={styles.backLink}>
            <ArrowLeft size={14} /> Back to site
          </Link>
          <ThemeToggle />
        </div>
      </nav>

      <div className={styles.container}>
        <header className={styles.header}>
          <h1 className="heading-gradient">Transaction Debugger</h1>
          <p className={styles.subtitle}>Decode envelopes and inspect transaction contexts</p>
        </header>

        <div className={styles.grid}>
          <section className={styles.panel}>
            <h2>Input Payload</h2>
            <div className={styles.inputGroup}>
              <label>Transaction Hash</label>
              <textarea
                className={styles.textarea}
                placeholder="64-character hex transaction hash..."
                rows={6}
                value={txHash}
                onChange={(e) => setTxHash(e.target.value)}
              />
            </div>
            <div className={styles.inputGroup}>
              <label htmlFor="network-select">Network</label>
              <select
                id="network-select"
                className={styles.select}
                value={network}
                onChange={(e) => setNetwork(e.target.value as Network)}
              >
                <option value="testnet">Testnet</option>
                <option value="mainnet">Mainnet</option>
                <option value="futurenet">Futurenet</option>
              </select>
            </div>
            <div className={styles.actions}>
              <Button variant="primary" onClick={handleDebug} disabled={loading}>
                {loading ? 'Executing...' : 'Decode & Fetch State'}
              </Button>
              <Button variant="ghost" onClick={() => { setTxHash(''); setResult(null); }}>Clear</Button>
            </div>
          </section>

          <section className={`${styles.panel} ${styles.resultPanel}`}>
            <h2>Decoded Context</h2>
            {!result ? (
              <div className={styles.emptyState}>
                <div className={styles.emptyIcon}>⬡</div>
                <p>Awaiting transaction payload...</p>
              </div>
            ) : (
              <div className={styles.resultContent}>
                {result.success ? (
                  <>
                    <h3 className={styles.statusSuccess}>
                      <CheckCircle2 size={18} /> Execution Succeeded
                    </h3>
                    <div className={styles.sectionLabel}>Output</div>
                    <pre className={styles.outputBlock}>{JSON.stringify(result.output, null, 2)}</pre>
                    {result.logs && (
                      <>
                        <div className={styles.sectionLabel}>Logs</div>
                        <pre className={styles.outputBlock}>{result.logs}</pre>
                      </>
                    )}
                    {result.callGraphSvg && (
                      <>
                        <div className={styles.sectionLabel}>Call Graph</div>
                        <div className={styles.callGraphFrame}>
                          {/* eslint-disable-next-line @next/next/no-img-element */}
                          <img src={svgToImageSrc(result.callGraphSvg)} alt="Contract call graph" />
                        </div>
                      </>
                    )}
                  </>
                ) : (
                  <>
                    <h3 className={styles.statusError}>
                      <XCircle size={18} /> Execution Failed
                    </h3>
                    <p><strong>Error:</strong> {result.error}</p>
                    {result.stderr && (
                      <>
                        <div className={styles.sectionLabel}>stderr</div>
                        <pre className={styles.errorBlock}>{result.stderr}</pre>
                      </>
                    )}
                    {result.stdout && (
                      <>
                        <div className={styles.sectionLabel}>stdout</div>
                        <pre className={styles.outputBlock}>{result.stdout}</pre>
                      </>
                    )}
                  </>
                )}
              </div>
            )}
          </section>
        </div>
      </div>
    </div>
  );
}
