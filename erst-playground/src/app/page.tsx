"use client";

import React, { useState } from 'react';
import Link from 'next/link';
import styles from './page.module.css';

const DEFAULT_CODE = `// SPDX-License-Identifier: Apache-2.0
#![no_std]
use soroban_sdk::{contract, contractimpl, symbol_short, vec, Env, Symbol, Vec};

#[contract]
pub struct HelloContract;

#[contractimpl]
impl HelloContract {
    pub fn hello(env: Env, to: Symbol) -> Vec<Symbol> {
        vec![&env, symbol_short!("Hello"), to]
    }
}
`;

export default function Playground() {
  const [code, setCode] = useState(DEFAULT_CODE);
  const [isRunning, setIsRunning] = useState(false);
  const [logs, setLogs] = useState<{time: string, level: string, msg: string}[]>([
    { time: "00:00:00", level: "info", msg: "Playground initialized." },
    { time: "00:00:00", level: "info", msg: "Ready for simulation." }
  ]);

  const handleRun = async () => {
    setIsRunning(true);
    setLogs(prev => [...prev, { time: new Date().toLocaleTimeString(), level: "info", msg: "Compiling WASM target via Erst Backend..." }]);
    
    try {
      const apiUrl = process.env.NEXT_PUBLIC_COMPILER_URL || 'http://localhost:8080/api/compile';
      const response = await fetch(apiUrl, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ code })
      });
      
      const data = await response.json();
      
      if (response.ok) {
        setLogs(prev => [...prev, { time: new Date().toLocaleTimeString(), level: "success", msg: "Build successful." }]);
        setLogs(prev => [...prev, { time: new Date().toLocaleTimeString(), level: "info", msg: `WASM binary generated (${data.wasmSize} bytes).` }]);
        if (data.logs) {
          setLogs(prev => [...prev, { time: new Date().toLocaleTimeString(), level: "info", msg: data.logs }]);
        }
      } else {
        setLogs(prev => [...prev, { time: new Date().toLocaleTimeString(), level: "error", msg: `Compilation failed: ${data.error}` }]);
        if (data.details) {
          setLogs(prev => [...prev, { time: new Date().toLocaleTimeString(), level: "error", msg: data.details }]);
        }
      }
    } catch (e: any) {
      setLogs(prev => [...prev, { time: new Date().toLocaleTimeString(), level: "error", msg: `Network Error: Could not connect to compile server.` }]);
    } finally {
      setIsRunning(false);
    }
  };

  return (
    <div className={styles.container}>
      <header className={styles.header}>
        <div className={styles.title}>
          <a href="https://erstt.xyz" style={{ color: 'inherit', textDecoration: 'none' }}>Erst.</a>
          <span style={{ color: '#52525b', margin: '0 0.5rem' }}>/</span>
          Playground <span className={styles.badge}>BETA</span>
        </div>
        <div className={styles.actions}>
          <button className={styles.btnSecondary} onClick={() => setLogs([])}>Clear</button>
          <button className={styles.btnRun} onClick={handleRun} disabled={isRunning}>
            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="3" strokeLinecap="round" strokeLinejoin="round">
              <polygon points="5 3 19 12 5 21 5 3"></polygon>
            </svg>
            {isRunning ? 'Running...' : 'Run Simulation'}
          </button>
        </div>
      </header>

      <main className={styles.workspace}>
        <div className={styles.editorPane}>
          <div className={styles.paneHeader}>
            src/lib.rs
          </div>
          <textarea 
            className={styles.editor}
            value={code}
            onChange={(e) => setCode(e.target.value)}
            spellCheck={false}
          />
        </div>
        <div className={styles.outputPane}>
          <div className={styles.paneHeader}>
            Terminal
          </div>
          <div className={styles.terminal}>
            {logs.map((log, i) => (
              <div key={i} className={styles.termLine}>
                <span className={styles.termTime}>[{log.time}]</span>
                <span className={
                  log.level === 'info' ? styles.termInfo :
                  log.level === 'success' ? styles.termSuccess :
                  log.level === 'error' ? styles.termError : styles.termWarn
                }>
                  {log.msg}
                </span>
              </div>
            ))}
            {isRunning && (
              <div className={styles.termLine}>
                <span className={styles.termTime}>[{new Date().toLocaleTimeString()}]</span>
                <span className={styles.termInfo}>...</span>
              </div>
            )}
          </div>
        </div>
      </main>
    </div>
  );
}
