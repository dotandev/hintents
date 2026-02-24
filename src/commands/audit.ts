import { Command } from 'commander';
import { createHash } from 'crypto';
import * as dotenv from 'dotenv';
import stringify from 'fast-json-stable-stringify';
import { AuditLogger } from '../audit/AuditLogger';
import { createAuditSigner } from '../audit/signing/factory';

// Load env for key/provider configuration
dotenv.config();

/**
 * Minimal audit command to demonstrate signer selection, including HSM/PKCS#11.
 *
 * This does not change the audit log format beyond including signature/publicKey metadata.
 */
export function registerAuditCommands(program: Command): void {
  program
    .command('audit:sign')
    .description('Generate a signed audit log from a JSON payload (demo/test utility)')
    .requiredOption('--payload <json>', 'JSON string to sign as the audit trace')
    .option('--dry-run', 'Validate payload/signer setup without generating a signature')
    .option('--hsm-provider <provider>', 'HSM provider to use (pkcs11). Defaults to software signing')
    .option(
      '--software-private-key <pem>',
      'Ed25519 private key (PKCS#8 PEM). If unset, uses ERST_AUDIT_PRIVATE_KEY_PEM'
    )
    .action(async (opts: any) => runAuditSign(opts));
}

type AuditSignIo = {
  stdout: Pick<NodeJS.WriteStream, 'write'>;
  stderr: Pick<NodeJS.WriteStream, 'write'>;
  exit: (code: number) => never;
};

type AuditSignOptions = {
  payload: string;
  dryRun?: boolean;
  hsmProvider?: string;
  softwarePrivateKey?: string;
};

export async function runAuditSign(
  opts: AuditSignOptions,
  io: AuditSignIo = { stdout: process.stdout, stderr: process.stderr, exit: process.exit }
): Promise<void> {
  try {
    const trace = JSON.parse(opts.payload);
    const canonicalString = stringify(trace);
    const traceHash = createHash('sha256').update(canonicalString).digest('hex');

    const signer = createAuditSigner({
      hsmProvider: opts.hsmProvider,
      softwarePrivateKeyPem: opts.softwarePrivateKey ?? process.env.ERST_AUDIT_PRIVATE_KEY_PEM,
    });

    if (opts.dryRun) {
      if (typeof signer.preflight === 'function') {
        await signer.preflight();
      }
      await signer.public_key();

      io.stdout.write(
        JSON.stringify(
          {
            dryRun: true,
            status: 'ok',
            signer: { provider: opts.hsmProvider ?? 'software' },
            validations: {
              payloadParsed: true,
              canonicalized: true,
              connectivityChecked: true,
            },
            traceHash,
          },
          null,
          2
        ) + '\n'
      );
      return;
    }

    const logger = new AuditLogger(signer, opts.hsmProvider ?? 'software');
    const log = await logger.generateLog(trace);

    // Print to stdout so callers can redirect to a file
    io.stdout.write(JSON.stringify(log, null, 2) + '\n');
  } catch (e) {
    const msg = e instanceof Error ? e.message : String(e);
    io.stderr.write(`[FAIL] audit signing failed: ${msg}\n`);
    io.exit(1);
  }
}
