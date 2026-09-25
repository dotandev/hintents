"use client";

import React, { useState } from 'react';
import { UploadCloud, CheckCircle2, AlertCircle, FileCode2, PackageOpen } from 'lucide-react';
import Link from 'next/link';

export default function PublishDashboard() {
  const [contractId, setContractId] = useState('');
  const [githubUrl, setGithubUrl] = useState('');
  const [step, setStep] = useState(1);

  return (
    <div className="max-w-4xl mx-auto px-6 py-12">
      <div className="mb-10">
        <h1 className="text-3xl font-bold mb-3">Publish Package</h1>
        <p className="text-zinc-400">Claim your deployed Soroban contract, verify its source code, and list it on the Erst Registry.</p>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-3 gap-8">
        {/* Progress Sidebar */}
        <div className="md:col-span-1 space-y-6">
          <div className={`flex items-center gap-3 ${step >= 1 ? 'text-zinc-100' : 'text-zinc-600'}`}>
            <div className={`w-8 h-8 rounded-full flex items-center justify-center font-bold text-sm ${step >= 1 ? 'bg-zinc-100 text-zinc-900' : 'bg-zinc-800'}`}>1</div>
            <span className="font-medium">Identify Contract</span>
          </div>
          <div className="w-0.5 h-6 bg-zinc-800 ml-4"></div>
          <div className={`flex items-center gap-3 ${step >= 2 ? 'text-zinc-100' : 'text-zinc-600'}`}>
            <div className={`w-8 h-8 rounded-full flex items-center justify-center font-bold text-sm ${step >= 2 ? 'bg-zinc-100 text-zinc-900' : 'bg-zinc-800'}`}>2</div>
            <span className="font-medium">Verify Ownership</span>
          </div>
          <div className="w-0.5 h-6 bg-zinc-800 ml-4"></div>
          <div className={`flex items-center gap-3 ${step >= 3 ? 'text-zinc-100' : 'text-zinc-600'}`}>
            <div className={`w-8 h-8 rounded-full flex items-center justify-center font-bold text-sm ${step >= 3 ? 'bg-zinc-100 text-zinc-900' : 'bg-zinc-800'}`}>3</div>
            <span className="font-medium">Link Source</span>
          </div>
        </div>

        {/* Form Content */}
        <div className="md:col-span-2 bg-zinc-900/40 border border-zinc-800 rounded-2xl p-8 shadow-xl">
          {step === 1 && (
            <div className="space-y-6 animate-in fade-in slide-in-from-bottom-4 duration-500">
              <div className="flex items-center gap-3 text-lg font-bold border-b border-zinc-800 pb-4">
                <PackageOpen className="text-emerald-400" /> Contract Details
              </div>
              
              <div>
                <label className="block text-sm font-medium text-zinc-300 mb-2">Contract ID (Mainnet or Testnet)</label>
                <input 
                  type="text" 
                  className="w-full bg-zinc-950 border border-zinc-700 rounded-lg px-4 py-3 text-zinc-100 focus:outline-none focus:border-zinc-500 transition-colors font-mono text-sm"
                  placeholder="C..."
                  value={contractId}
                  onChange={(e) => setContractId(e.target.value)}
                />
                <p className="text-xs text-zinc-500 mt-2">The contract must already be deployed to the Stellar network.</p>
              </div>

              <div>
                <label className="block text-sm font-medium text-zinc-300 mb-2">Human-Readable Name</label>
                <div className="flex">
                  <span className="inline-flex items-center px-4 rounded-l-lg border border-r-0 border-zinc-700 bg-zinc-900 text-zinc-500">@</span>
                  <input 
                    type="text" 
                    className="flex-1 bg-zinc-950 border border-zinc-700 rounded-r-lg px-4 py-3 text-zinc-100 focus:outline-none focus:border-zinc-500 transition-colors"
                    placeholder="organization/package-name"
                  />
                </div>
              </div>

              <button 
                onClick={() => setStep(2)}
                className="w-full py-3 bg-zinc-100 text-zinc-900 hover:bg-white rounded-lg font-bold transition-colors mt-4"
              >
                Continue
              </button>
            </div>
          )}

          {step === 2 && (
            <div className="space-y-6 animate-in fade-in slide-in-from-bottom-4 duration-500">
              <div className="flex items-center gap-3 text-lg font-bold border-b border-zinc-800 pb-4">
                <ShieldCheck className="text-blue-400" /> Verify Ownership
              </div>
              
              <div className="bg-blue-500/10 border border-blue-500/20 rounded-xl p-4 flex gap-3 text-blue-200 text-sm">
                <AlertCircle className="w-5 h-5 flex-shrink-0 mt-0.5 text-blue-400" />
                <p>To claim this contract, you must sign a cryptographic challenge proving you control the exact Stellar account that deployed <span className="font-mono text-blue-300">{contractId || 'the contract'}</span>.</p>
              </div>

              <div className="bg-zinc-950 border border-zinc-800 rounded-xl p-6 text-center">
                <p className="text-zinc-400 mb-6">Your currently connected wallet does not match the deployer address.</p>
                <button 
                  onClick={() => setStep(3)} // Bypassing for mockup
                  className="px-6 py-2 bg-zinc-800 hover:bg-zinc-700 text-zinc-100 rounded-lg font-medium transition-colors border border-zinc-700"
                >
                  Sign Challenge (Mock)
                </button>
              </div>
            </div>
          )}

          {step === 3 && (
            <div className="space-y-6 animate-in fade-in slide-in-from-bottom-4 duration-500">
              <div className="flex items-center gap-3 text-lg font-bold border-b border-zinc-800 pb-4">
                <FileCode2 className="text-pink-400" /> Link Source Code
              </div>
              
              <p className="text-sm text-zinc-400">Linking your source code allows the registry to verify that the deployed WASM byte-for-byte matches the Rust source code.</p>
              
              <div>
                <label className="block text-sm font-medium text-zinc-300 mb-2">GitHub Repository URL</label>
                <input 
                  type="text" 
                  className="w-full bg-zinc-950 border border-zinc-700 rounded-lg px-4 py-3 text-zinc-100 focus:outline-none focus:border-zinc-500 transition-colors"
                  placeholder="https://github.com/..."
                  value={githubUrl}
                  onChange={(e) => setGithubUrl(e.target.value)}
                />
              </div>

              <div className="border-2 border-dashed border-zinc-800 rounded-xl p-8 text-center hover:bg-zinc-800/30 transition-colors cursor-pointer">
                <UploadCloud className="w-8 h-8 text-zinc-500 mx-auto mb-3" />
                <p className="text-zinc-300 font-medium">Or upload compilation artifacts</p>
                <p className="text-zinc-500 text-xs mt-1">Drop .wasm and .json files here</p>
              </div>

              <div className="flex gap-4 pt-4">
                <button 
                  onClick={() => setStep(2)}
                  className="flex-1 py-3 bg-zinc-900 text-zinc-300 hover:bg-zinc-800 rounded-lg font-medium transition-colors border border-zinc-800"
                >
                  Back
                </button>
                <Link href="/" className="flex-1 py-3 bg-zinc-100 text-zinc-900 hover:bg-white text-center rounded-lg font-bold transition-colors">
                  Publish Package
                </Link>
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
