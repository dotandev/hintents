import { Command } from 'commander';
import { registerAuditCommands } from '../audit';
import { createAuditSigner } from '../../audit/signing/factory';
import { AuditLogger } from '../../audit/AuditLogger';

jest.mock('../../audit/signing/factory', () => ({
  createAuditSigner: jest.fn(),
}));

jest.mock('../../audit/AuditLogger', () => ({
  AuditLogger: jest.fn().mockImplementation(() => ({
    generateLog: jest.fn().mockResolvedValue({ ok: true }),
  })),
}));

describe('audit:sign --dry-run', () => {
  let program: Command;

  beforeEach(() => {
    program = new Command();
    registerAuditCommands(program);
    jest.clearAllMocks();
  });

  test('validates payload and connectivity without invoking signing logger', async () => {
    (createAuditSigner as jest.Mock).mockReturnValue({
      sign: jest.fn(),
      public_key: jest.fn().mockResolvedValue('-----BEGIN PUBLIC KEY-----\\nabc\\n-----END PUBLIC KEY-----'),
      attestation_chain: jest.fn().mockResolvedValue(undefined),
    });

    const stdoutSpy = jest.spyOn(process.stdout, 'write').mockImplementation(() => true);

    await program.parseAsync([
      'node',
      'test',
      'audit:sign',
      '--payload',
      '{"input":{},"state":{},"events":[],"timestamp":"2026-01-01T00:00:00.000Z"}',
      '--hsm-provider',
      'pkcs11',
      '--dry-run',
    ]);

    expect(createAuditSigner).toHaveBeenCalledTimes(1);
    expect(AuditLogger).not.toHaveBeenCalled();
    expect(stdoutSpy).toHaveBeenCalledWith(expect.stringContaining('"dry_run": true'));
    expect(stdoutSpy).toHaveBeenCalledWith(expect.stringContaining('"signer_provider": "pkcs11"'));

    stdoutSpy.mockRestore();
  });

  test('returns failure for invalid payload json', async () => {
    const consoleErrorSpy = jest.spyOn(console, 'error').mockImplementation();
    const processExitSpy = jest.spyOn(process, 'exit').mockImplementation((() => undefined) as any);

    await program.parseAsync([
      'node',
      'test',
      'audit:sign',
      '--payload',
      '{not-json}',
      '--dry-run',
    ]);

    expect(consoleErrorSpy).toHaveBeenCalledWith(expect.stringContaining('[FAIL] audit signing failed'));
    expect(processExitSpy).toHaveBeenCalledWith(1);

    consoleErrorSpy.mockRestore();
    processExitSpy.mockRestore();
  });

  test('still performs normal signing flow when dry-run is not set', async () => {
    (createAuditSigner as jest.Mock).mockReturnValue({
      sign: jest.fn(),
      public_key: jest.fn().mockResolvedValue('pem'),
      attestation_chain: jest.fn().mockResolvedValue(undefined),
    });

    const stdoutSpy = jest.spyOn(process.stdout, 'write').mockImplementation(() => true);

    await program.parseAsync([
      'node',
      'test',
      'audit:sign',
      '--payload',
      '{"input":{},"state":{},"events":[],"timestamp":"2026-01-01T00:00:00.000Z"}',
    ]);

    expect(AuditLogger).toHaveBeenCalledTimes(1);

    stdoutSpy.mockRestore();
  });
});
