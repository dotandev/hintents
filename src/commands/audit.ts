// Copyright (c) Hintents Authors.
// SPDX-License-Identifier: Apache-2.0

import { Command } from 'commander';
import * as dotenv from 'dotenv';
import * as fs from 'fs';
import { AuditLogger } from '../audit/AuditLogger';
import { renderAuditHTML, writeAuditReport } from '../audit/AuditRenderer';
import { createAuditSigner } from '../audit/signing/factory';
import { verifyAuditLog } from '../audit/AuditVerifier';

// Load env for key/provider configuration
dotenv.config();

/**
 * Audit command that supports software (Ed25519), PKCS#11, and AWS KMS signing.
 *
 * Provider selection:
 *   --hsm-provider software   (default) local Ed25519 PKCS#8 PEM key
 *   --hsm-provider pkcs11     PKCS#11 HSM via pkcs11js (see PKCS#11 env vars)
 *   --hsm-provider kms        AWS KMS asymmetric key (see KMS env vars)
 *
 * KMS env vars:
 *   ERST_KMS_KEY_ID             KMS key ID or ARN
 *   AWS_REGION                  AWS region
 *   ERST_KMS_SIGNING_ALGORITHM  KMS algorithm (default: ECDSA_SHA_256)
 */
export function registerAuditCommands(program: Command): void {
  program
    .command('audit:sign')
    .description('Generate a signed audit log from a JSON payload')
    .requiredOption('--payload <json>', 'JSON string to sign as the audit trace')
    .option(
      '--hsm-provider <provider>',
      'Signing provider: software (default), pkcs11, or kms'
    )
    .option(
      '--software-private-key <pem>',
      'Ed25519 private key (PKCS#8 PEM). If unset, uses ERST_AUDIT_PRIVATE_KEY_PEM'
    )
    .option(
      '--kms-key-id <id>',
      'AWS KMS key ID or ARN. If unset, uses ERST_KMS_KEY_ID'
    )
    .option(
      '--kms-signing-algorithm <alg>',
      'AWS KMS signing algorithm (default: ECDSA_SHA_256). If unset, uses ERST_KMS_SIGNING_ALGORITHM'
    )
    .action(async (opts: any) => {
      try {
        const trace = JSON.parse(opts.payload);

        const signer = createAuditSigner({
          hsmProvider: opts.hsmProvider,
          softwarePrivateKeyPem: opts.softwarePrivateKey ?? process.env.ERST_AUDIT_PRIVATE_KEY_PEM,
          kmsKeyId: opts.kmsKeyId,
          kmsSigningAlgorithm: opts.kmsSigningAlgorithm,
        });

        const providerLabel = opts.hsmProvider ?? 'software';
        const logger = new AuditLogger(signer, providerLabel);
        const log = await logger.generateLog(trace);

        // Print to stdout so callers can redirect to a file
        process.stdout.write(JSON.stringify(log, null, 2) + '\n');
      } catch (e) {
        const msg = e instanceof Error ? e.message : String(e);
        console.error(`[FAIL] audit signing failed: ${msg}`);
        process.exit(1);
      }
    });

  program
    .command('audit:render')
    .description('Render a raw ExecutionTrace or SignedAuditLog JSON payload to an HTML report')
    .requiredOption('--payload <json>', 'JSON string containing the audit payload (ExecutionTrace or SignedAuditLog)')
    .option('--output <path>', 'Write HTML to this file instead of stdout')
    .option('--title <title>', 'Report title (default: "Audit Report")')
    .action((opts: any) => {
      try {
        const payload = JSON.parse(opts.payload);

        if (opts.output) {
          writeAuditReport(payload, opts.output, opts.title);
          console.error(`[OK] Audit report written to ${opts.output}`);
        } else {
          process.stdout.write(renderAuditHTML(payload, opts.title));
        }
      } catch (e) {
        const msg = e instanceof Error ? e.message : String(e);
        console.error(`[FAIL] audit render failed: ${msg}`);
    .command('audit:verify')
    .description('Verify an audit log signature locally (offline verification)')
    .option('--payload <json>', 'JSON string of the audit trace')
    .option('--sig <hex>', 'Hex-encoded signature')
    .option('--pubkey <pem>', 'Public key in PEM format')
    .option('--file <path>', 'Path to a complete audit log JSON file')
    .action(async (opts: any) => {
      try {
        let auditLog: any;

        if (opts.file) {
          const content = fs.readFileSync(opts.file, 'utf8');
          auditLog = JSON.parse(content);
        } else if (opts.payload && opts.sig && opts.pubkey) {
          // Reconstruct enough of the log to verify
          // verifyAuditLog calculates the hash from the trace
          auditLog = {
            trace: JSON.parse(opts.payload),
            signature: opts.sig,
            publicKey: opts.pubkey,
            // Re-calculate hash here because verifyAuditLog expects it to exist and match
            // However, verifyAuditLog also re-calculates it.
            // Let's look at the implementation of verifyAuditLog again.
          };

          // Re-calculate the hash to satisfy the verifyAuditLog structure
          const stringify = (await import('fast-json-stable-stringify')).default;
          const { createHash } = await import('crypto');
          const canonicalString = stringify(auditLog.trace);
          auditLog.hash = createHash('sha256').update(canonicalString).digest('hex');
        } else {
          throw new Error('You must provide either --file or all of (--payload, --sig, --pubkey)');
        }

        const isValid = verifyAuditLog(auditLog);

        if (isValid) {
          console.log('[OK] Verification successful: Signature and integrity verified.');
        } else {
          console.error('[FAIL] Verification failed: Invalid signature or tampered payload.');
          process.exit(1);
        }
      } catch (e) {
        const msg = e instanceof Error ? e.message : String(e);
        console.error(`[FAIL] audit verification failed: ${msg}`);
        process.exit(1);
      }
    });
}
