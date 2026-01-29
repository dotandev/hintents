import { generateKeyPairSync, KeyObject } from 'crypto';

// Use Ed25519 as requested in the Must-haves
export const generateAuditKeys = () => {
  const { publicKey, privateKey } = generateKeyPairSync('ed25519', {
    publicKeyEncoding: { type: 'spki', format: 'pem' },
    privateKeyEncoding: { type: 'pkcs8', format: 'pem' },
  });

  return { publicKey, privateKey };
};