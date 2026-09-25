import React from 'react';

export const metadata = {
  title: 'Erst - High-Fidelity Glass-Box Debugging for Stellar',
  description: 'Erst eliminates the opaque black box experience of failed smart contract transactions on the Stellar network.',
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html lang="en">
      <body style={{ margin: 0, padding: 0, backgroundColor: '#09090b' }}>
        {children}
      </body>
    </html>
  );
}
