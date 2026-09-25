import React from 'react';

export const metadata = {
  title: 'Erst Playground - Interactive Soroban Environment',
  description: 'Write, compile, and simulate Stellar Soroban smart contracts directly in the browser.',
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html lang="en">
      <body style={{ margin: 0, padding: 0, backgroundColor: '#09090b', color: '#fafafa', fontFamily: 'Inter, sans-serif' }}>
        {children}
      </body>
    </html>
  );
}
