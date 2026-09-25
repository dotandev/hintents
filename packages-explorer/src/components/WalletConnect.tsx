"use client";

import React, { useState, useEffect } from 'react';
import { isConnected, isAllowed, setAllowed, getUserInfo } from '@stellar/freighter-api';
import { Wallet } from 'lucide-react';

export default function WalletConnect() {
  const [pubKey, setPubKey] = useState<string | null>(null);
  const [isFreighterInstalled, setIsFreighterInstalled] = useState(false);

  useEffect(() => {
    const checkConnection = async () => {
      const connected = await isConnected();
      setIsFreighterInstalled(connected);
      
      if (connected) {
        const allowed = await isAllowed();
        if (allowed) {
          const info = await getUserInfo();
          if (info.publicKey) {
            setPubKey(info.publicKey);
          }
        }
      }
    };
    checkConnection();
  }, []);

  const connectWallet = async () => {
    if (!isFreighterInstalled) {
      alert("Please install the Freighter browser extension to log in.");
      return;
    }
    try {
      await setAllowed();
      const info = await getUserInfo();
      if (info.publicKey) {
        setPubKey(info.publicKey);
      }
    } catch (e) {
      console.error("User denied connection or error occurred:", e);
    }
  };

  const truncateKey = (key: string) => {
    return `${key.slice(0, 5)}...${key.slice(-4)}`;
  };

  return (
    <button 
      onClick={connectWallet}
      className="flex items-center gap-2 px-4 py-2 bg-[#27272a] hover:bg-[#3f3f46] transition-colors rounded-md text-sm text-[#fafafa] border border-[#3f3f46]"
    >
      <Wallet size={16} />
      {pubKey ? truncateKey(pubKey) : "Connect Wallet"}
    </button>
  );
}
