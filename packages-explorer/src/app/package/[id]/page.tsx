// Copyright (c) Hintents Authors.
// SPDX-License-Identifier: Apache-2.0

"use client";

import React, { useState } from 'react';
import { ArrowLeft, ShieldCheck, Copy, Code2, Box, Activity } from 'lucide-react';
import Link from 'next/link';

// Mock data matching the dashboard
const PACKAGE_DATA = {
  id: 'C_ASSET_TOKEN_192837',
  name: 'Soroban Token Standard',
  author: 'SDF',
  version: 'v1.0.2',
  verified: true,
  txs: '1.2M',
  description: 'The official reference implementation for the Soroban token standard (SEP-41). Use this to deploy standard fungible tokens or build AMMs.',
  functions: [
    { name: 'initialize', args: ['admin: Address', 'decimal: u32', 'name: String', 'symbol: String'], returns: 'Void' },
    { name: 'balance', args: ['id: Address'], returns: 'i128' },
    { name: 'transfer', args: ['from: Address', 'to: Address', 'amount: i128'], returns: 'Void' },
    { name: 'mint', args: ['to: Address', 'amount: i128'], returns: 'Void' }
  ]
};

export default function PackageDetail({ params }: { params: { id: string } }) {
  const [activeTab, setActiveTab] = useState('abi');

  return (
    <div className="max-w-7xl mx-auto px-6 py-12">
      <Link href="/" className="inline-flex items-center gap-2 text-sm text-zinc-400 hover:text-zinc-100 transition-colors mb-8">
        <ArrowLeft size={16} /> Back to Registry
      </Link>

      {/* Header */}
      <div className="flex flex-col md:flex-row md:items-start justify-between gap-6 mb-12">
        <div>
          <div className="flex items-center gap-3 mb-2">
            <h1 className="text-3xl font-bold">{PACKAGE_DATA.name}</h1>
            {PACKAGE_DATA.verified && (
              <span className="flex items-center gap-1 px-2 py-1 rounded bg-blue-500/10 text-blue-400 text-xs font-medium border border-blue-500/20">
                <ShieldCheck size={14} /> Verified Source
              </span>
            )}
          </div>
          <div className="flex items-center gap-4 text-sm text-zinc-400">
            <span>by <span className="text-zinc-200">{PACKAGE_DATA.author}</span></span>
            <span>•</span>
            <span className="font-mono bg-zinc-900 px-2 py-1 rounded text-zinc-300 flex items-center gap-2 border border-zinc-800">
              {PACKAGE_DATA.id}
              <Copy size={12} className="cursor-pointer hover:text-zinc-100" />
            </span>
          </div>
          <p className="mt-4 text-zinc-400 max-w-2xl">{PACKAGE_DATA.description}</p>
        </div>
        
        <div className="flex gap-3">
          <button className="px-6 py-2 bg-zinc-100 text-zinc-900 hover:bg-white rounded-lg font-medium transition-colors">
            Use in Playground
          </button>
        </div>
      </div>

      {/* Tabs */}
      <div className="border-b border-zinc-800 mb-8">
        <div className="flex gap-8">
          <button 
            onClick={() => setActiveTab('abi')}
            className={`pb-4 text-sm font-medium transition-colors border-b-2 ${activeTab === 'abi' ? 'border-zinc-100 text-zinc-100' : 'border-transparent text-zinc-500 hover:text-zinc-300'}`}
          >
            <div className="flex items-center gap-2"><Box size={16}/> ABI & Functions</div>
          </button>
          <button 
            onClick={() => setActiveTab('source')}
            className={`pb-4 text-sm font-medium transition-colors border-b-2 ${activeTab === 'source' ? 'border-zinc-100 text-zinc-100' : 'border-transparent text-zinc-500 hover:text-zinc-300'}`}
          >
            <div className="flex items-center gap-2"><Code2 size={16}/> Source Code</div>
          </button>
          <button 
            onClick={() => setActiveTab('metrics')}
            className={`pb-4 text-sm font-medium transition-colors border-b-2 ${activeTab === 'metrics' ? 'border-zinc-100 text-zinc-100' : 'border-transparent text-zinc-500 hover:text-zinc-300'}`}
          >
            <div className="flex items-center gap-2"><Activity size={16}/> Metrics</div>
          </button>
        </div>
      </div>

      {/* Tab Content */}
      {activeTab === 'abi' && (
        <div className="bg-zinc-900/30 border border-zinc-800 rounded-xl overflow-hidden">
          <div className="px-6 py-4 border-b border-zinc-800 bg-zinc-900/50">
            <h3 className="font-medium">Exported Functions</h3>
          </div>
          <div className="divide-y divide-zinc-800/50">
            {PACKAGE_DATA.functions.map((fn, i) => (
              <div key={i} className="p-6 hover:bg-zinc-800/20 transition-colors">
                <div className="flex items-center gap-2 mb-3">
                  <span className="text-pink-400 font-mono text-sm">fn</span>
                  <span className="font-bold text-zinc-200 font-mono">{fn.name}</span>
                </div>
                <div className="bg-zinc-950 rounded-lg p-4 border border-zinc-800 font-mono text-sm">
                  <div className="text-zinc-500 mb-1">// Parameters</div>
                  {fn.args.length > 0 ? (
                    fn.args.map((arg, j) => (
                      <div key={j} className="pl-4">
                        <span className="text-zinc-300">{arg.split(':')[0]}</span>
                        <span className="text-zinc-500">:</span>
                        <span className="text-blue-400">{arg.split(':')[1]}</span>
                      </div>
                    ))
                  ) : (
                    <div className="pl-4 text-zinc-500">None</div>
                  )}
                  <div className="text-zinc-500 mt-3 mb-1">// Returns</div>
                  <div className="pl-4 text-emerald-400">{fn.returns}</div>
                </div>
              </div>
            ))}
          </div>
        </div>
      )}

      {activeTab === 'source' && (
        <div className="bg-zinc-900/30 border border-zinc-800 rounded-xl p-8 text-center text-zinc-500">
          <Code2 size={32} className="mx-auto mb-4 opacity-50" />
          <p>Verified source code linking to GitHub will appear here.</p>
        </div>
      )}
    </div>
  );
}
