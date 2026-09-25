import './globals.css';
import React from 'react';
import WalletConnect from '../components/WalletConnect';

export const metadata = {
  title: 'Stellar Packages Explorer',
  description: 'Explore Soroban smart contracts, ABIs, and network modules.',
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en">
      <body className="antialiased min-h-screen flex flex-col font-sans bg-zinc-950 text-zinc-50">
        <nav className="sticky top-0 z-50 w-full border-b border-zinc-800/50 bg-zinc-950/80 backdrop-blur-md">
          <div className="max-w-7xl mx-auto px-6 h-16 flex items-center justify-between">
            <div className="flex items-center gap-2">
              <span className="font-bold text-xl tracking-tight">Erst Registry</span>
              <span className="text-xs font-medium px-2 py-0.5 rounded-full bg-zinc-800 text-zinc-300 ml-2 border border-zinc-700">TESTNET</span>
            </div>
            <div className="hidden md:flex items-center gap-6 text-sm font-medium text-zinc-400">
              <a href="#" className="hover:text-zinc-50 transition-colors">Contracts</a>
              <a href="/publish" className="hover:text-zinc-50 transition-colors">Publish</a>
              <a href="#" className="hover:text-zinc-50 transition-colors">Docs</a>
              <WalletConnect />
            </div>
          </div>
        </nav>
        <main className="flex-grow">
          {children}
        </main>
      </body>
    </html>
  );
}
