"use client";

import React from 'react';
import Link from 'next/link';
import styles from './page.module.css';

export default function LandingPage() {
  return (
    <div className={styles.container}>
      {/* Navigation */}
      <nav className={styles.nav}>
        <div className={styles.navContent}>
          <div className={styles.navBrand}>
            <span className={styles.navLogo}>Erst.</span>
            <span className={styles.navVersion}>v0.1.0</span>
          </div>
          <div className={styles.navLinks}>
            <Link href="#features">Features</Link>
            <a href="https://dotandev-hintents-75.mintlify.app/" target="_blank" rel="noopener noreferrer">
              Documentation
            </a>
            <Link href="https://playground.erstt.xyz">Playground</Link>
            <a href="https://crates.io/crates/simulator" target="_blank" rel="noopener noreferrer">
              Packages
            </a>
            <a href="https://github.com/dotandev/hintents" target="_blank" rel="noopener noreferrer" aria-label="GitHub">
              <svg width="20" height="20" fill="currentColor" viewBox="0 0 24 24" aria-hidden="true">
                <path fillRule="evenodd" d="M12 2C6.477 2 2 6.484 2 12.017c0 4.425 2.865 8.18 6.839 9.504.5.092.682-.217.682-.483 0-.237-.008-.868-.013-1.703-2.782.605-3.369-1.343-3.369-1.343-.454-1.158-1.11-1.466-1.11-1.466-.908-.62.069-.608.069-.608 1.003.07 1.531 1.032 1.531 1.032.892 1.53 2.341 1.088 2.91.832.092-.647.35-1.088.636-1.338-2.22-.253-4.555-1.113-4.555-4.951 0-1.093.39-1.988 1.029-2.688-.103-.253-.446-1.272.098-2.65 0 0 .84-.27 2.75 1.026A9.564 9.564 0 0112 6.844c.85.004 1.705.115 2.504.337 1.909-1.296 2.747-1.027 2.747-1.027.546 1.379.202 2.398.1 2.651.64.7 1.028 1.595 1.028 2.688 0 3.848-2.339 4.695-4.566 4.943.359.309.678.92.678 1.855 0 1.338-.012 2.419-.012 2.747 0 .268.18.58.688.482A10.019 10.019 0 0022 12.017C22 6.484 17.522 2 12 2z" clipRule="evenodd"></path>
              </svg>
            </a>
          </div>
        </div>
      </nav>

      {/* Hero Section */}
      <main className={styles.main}>
        <div className={styles.heroContent}>
          {/* Hero Copy */}
          <div className={styles.heroCopy}>
            <h1 className={styles.heroTitle}>
              High-Fidelity <br />
              <span className={styles.textMuted}>Glass-Box Debugging</span> <br />
              for Stellar.
            </h1>
            <p className={styles.heroSubtitle}>
              Erst eliminates the opaque "black box" experience of failed smart contract transactions. Map generic network errors back to human-readable diagnostics, Rust source code, and exact execution traces.
            </p>
            <div className={styles.ctaGroup}>
              <a href="https://dotandev-hintents-75.mintlify.app/" className={styles.btnPrimary}>
                Read the Docs
              </a>
              <Link href="https://playground.erstt.xyz" className={styles.btnSecondary}>
                Open Playground
              </Link>
            </div>
          </div>

          {/* Terminal / Installation */}
          <div className={styles.terminal}>
            <div className={styles.terminalHeader}>
              <div className={styles.terminalDots}>
                <div className={styles.terminalDot}></div>
                <div className={styles.terminalDot}></div>
                <div className={styles.terminalDot}></div>
              </div>
              <div className={styles.terminalTitle}>install-erst.sh</div>
            </div>
            <div className={styles.terminalBody}>
              <div className={styles.comment}># 1. Install via Homebrew</div>
              <div className={styles.command}>
                <span className={styles.prompt}>$</span>
                <span className={styles.input}>brew install dotandev/hintents/erst</span>
              </div>
              
              <div className={styles.comment} style={{ marginTop: '1.5rem' }}># 2. Debug a failed transaction on testnet</div>
              <div className={styles.command}>
                <span className={styles.prompt}>$</span>
                <span className={styles.input}>erst debug &lt;tx-hash&gt; --network testnet</span>
              </div>

              <div className={styles.output}>
                <span className={styles.success}>✔</span> Fetched transaction envelope...<br />
                <span className={styles.success}>✔</span> Downloaded ledger state...<br />
                <span className={styles.info}>ℹ</span> Replaying logically in local simulator...
              </div>
              
              <div className={styles.errorBlock}>
                <span className={styles.errorText}>Error:</span> Trapped at <span className={styles.input}>src/contract.rs:42</span><br />
                <span className={styles.errorContext}>HostError: Contract execution panicked</span>
              </div>
            </div>
          </div>
        </div>
      </main>

      {/* Details / Features Section */}
      <section id="features" className={styles.features}>
        <div className={styles.featuresContent}>
          <div className={styles.featuresHeader}>
            <h2>Engineering over guesswork.</h2>
            <p>Stop relying on print statements. Erst provides a complete suite of tools to trace, profile, and verify Soroban smart contracts locally.</p>
          </div>
          
          <div className={styles.featuresGrid}>
            <div className={styles.featureCard}>
              <div className={styles.featureIcon}>
                <svg fill="none" stroke="currentColor" viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"></path>
                </svg>
              </div>
              <h3>Local Simulation</h3>
              <p>
                Fetch transaction envelopes and ledger states directly from an RPC provider, and re-execute them logically in an isolated local environment without spending lumens.
              </p>
            </div>

            <div className={styles.featureCard}>
              <div className={styles.featureIcon}>
                <svg fill="none" stroke="currentColor" viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10 20l4-16m4 4l4 4-4 4M6 16l-4-4 4-4"></path>
                </svg>
              </div>
              <h3>Trace Decoding</h3>
              <p>
                Map execution steps, Host environment metrics, and WASM instruction failures directly back to your readable Rust source lines using embedded DWARF debug symbols.
              </p>
            </div>

            <div className={styles.featureCard}>
              <div className={styles.featureIcon}>
                <svg fill="none" stroke="currentColor" viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M20 7l-8-4-8 4m16 0l-8 4m8-4v10l-8 4m0-10L4 7m8 4v10M4 7v10l8 4"></path>
                </svg>
              </div>
              <h3>VS Code Integration</h3>
              <p>
                A native DAP (Debug Adapter Protocol) extension that turns your editor into a powerful time-travel debugger. Inspect stack traces and state changes without leaving your IDE.
              </p>
            </div>
          </div>
        </div>
      </section>

      {/* Footer */}
      <footer className={styles.footer}>
        <div className={styles.footerContent}>
          <div className={styles.footerBrand}>
            <span className={styles.footerLogo}>Erst.</span>
            <span className={styles.footerCopyright}>© 2026 Hintents</span>
          </div>
          <div className={styles.navLinks}>
            <a href="https://crates.io/crates/simulator" target="_blank" rel="noopener noreferrer">Crates.io</a>
            <a href="https://github.com/dotandev/hintents/releases" target="_blank" rel="noopener noreferrer">Releases</a>
            <a href="https://dotandev-hintents-75.mintlify.app/" target="_blank" rel="noopener noreferrer">Documentation</a>
            <a href="https://github.com/dotandev/hintents" target="_blank" rel="noopener noreferrer">GitHub</a>
          </div>
        </div>
      </footer>
    </div>
  );
}
