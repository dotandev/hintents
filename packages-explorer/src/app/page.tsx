"use client";

import React, { useState } from 'react';
import { Search, Code2, Box, ArrowRight, ShieldCheck } from 'lucide-react';

const MOCK_PACKAGES = [
  { id: 'C_ASSET_TOKEN_192837', name: 'Soroban Token Standard', author: 'SDF', txs: '1.2M', version: 'v1.0.2', verified: true },
  { id: 'C_AMM_LIQUIDITY_POOL', name: 'Phoenix AMM Core', author: 'PhoenixDex', txs: '840K', version: 'v2.1.0', verified: true },
  { id: 'C_SMART_WALLET_FACTORY', name: 'Multi-sig Wallet Factory', author: 'WalletConnect', txs: '12K', version: 'v0.9.5', verified: false },
  { id: 'C_ORACLE_PRICE_FEED', name: 'Band Protocol Oracle', author: 'Band', txs: '3.4M', version: 'v3.0.0', verified: true },
];

export default function RegistryHome() {
  const [search, setSearch] = useState('');

  return (
    <div className="max-w-7xl mx-auto px-6 py-12">
      {/* Search Hero */}
      <div className="flex flex-col items-center justify-center py-20 text-center">
        <h1 className="text-4xl md:text-5xl font-bold tracking-tight mb-6">
          Explore Stellar Smart Contracts
        </h1>
        <p className="text-lg text-zinc-400 mb-10 max-w-2xl">
          The central registry for Soroban packages, ABIs, and verified WASM binaries. Search by Contract ID, name, or developer.
        </p>
        
        <div className="w-full max-w-3xl relative">
          <div className="absolute inset-y-0 left-0 pl-4 flex items-center pointer-events-none">
            <Search className="h-5 w-5 text-zinc-500" />
          </div>
          <input
            type="text"
            className="block w-full pl-12 pr-4 py-4 bg-zinc-900 border border-zinc-700 rounded-xl text-zinc-100 placeholder-zinc-500 focus:outline-none focus:ring-2 focus:ring-zinc-600 focus:border-transparent transition-all shadow-lg"
            placeholder="Search contracts e.g., C... or 'Token'"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
          />
          <div className="absolute inset-y-0 right-0 pr-2 flex items-center">
            <button className="bg-zinc-800 hover:bg-zinc-700 text-zinc-300 px-4 py-2 rounded-lg text-sm font-medium transition-colors">
              Search
            </button>
          </div>
        </div>
      </div>

      {/* Stats/Metrics */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-6 mb-16">
        <div className="bg-zinc-900/50 border border-zinc-800 rounded-xl p-6 flex items-center gap-4">
          <div className="p-3 bg-zinc-800 rounded-lg">
            <Box className="w-6 h-6 text-zinc-300" />
          </div>
          <div>
            <div className="text-2xl font-bold">14,209</div>
            <div className="text-sm text-zinc-400">Total Packages</div>
          </div>
        </div>
        <div className="bg-zinc-900/50 border border-zinc-800 rounded-xl p-6 flex items-center gap-4">
          <div className="p-3 bg-zinc-800 rounded-lg">
            <Code2 className="w-6 h-6 text-zinc-300" />
          </div>
          <div>
            <div className="text-2xl font-bold">3,842</div>
            <div className="text-sm text-zinc-400">Verified Sources</div>
          </div>
        </div>
        <div className="bg-zinc-900/50 border border-zinc-800 rounded-xl p-6 flex items-center gap-4">
          <div className="p-3 bg-zinc-800 rounded-lg">
            <ArrowRight className="w-6 h-6 text-zinc-300" />
          </div>
          <div>
            <div className="text-2xl font-bold">89.4M</div>
            <div className="text-sm text-zinc-400">Contract Invocations</div>
          </div>
        </div>
      </div>

      {/* Trending / Recent Packages */}
      <div>
        <div className="flex items-center justify-between mb-6">
          <h2 className="text-xl font-bold">Trending Packages</h2>
          <button className="text-sm text-zinc-400 hover:text-zinc-100 transition-colors flex items-center gap-1">
            View all <ArrowRight className="w-4 h-4" />
          </button>
        </div>
        
        <div className="bg-zinc-900/30 border border-zinc-800 rounded-xl overflow-hidden">
          <table className="w-full text-left border-collapse">
            <thead>
              <tr className="border-b border-zinc-800 bg-zinc-900/50 text-sm text-zinc-400">
                <th className="px-6 py-4 font-medium">Package Name</th>
                <th className="px-6 py-4 font-medium">Contract ID</th>
                <th className="px-6 py-4 font-medium">Version</th>
                <th className="px-6 py-4 font-medium text-right">Invocations</th>
              </tr>
            </thead>
            <tbody className="text-sm divide-y divide-zinc-800/50">
              {MOCK_PACKAGES.map((pkg, i) => (
                <tr key={i} className="hover:bg-zinc-800/20 transition-colors cursor-pointer">
                  <td className="px-6 py-4">
                    <div className="flex items-center gap-2">
                      <span className="font-medium text-zinc-100">{pkg.name}</span>
                      {pkg.verified && <ShieldCheck className="w-4 h-4 text-blue-400" />}
                    </div>
                    <div className="text-xs text-zinc-500 mt-1">by {pkg.author}</div>
                  </td>
                  <td className="px-6 py-4 font-mono text-zinc-400 text-xs">
                    {pkg.id}
                  </td>
                  <td className="px-6 py-4">
                    <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-zinc-800 text-zinc-300">
                      {pkg.version}
                    </span>
                  </td>
                  <td className="px-6 py-4 text-right text-zinc-400">
                    {pkg.txs}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
}
