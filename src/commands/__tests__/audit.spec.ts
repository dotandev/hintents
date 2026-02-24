// Copyright (c) Hintents Authors.
// SPDX-License-Identifier: Apache-2.0

import { Command } from 'commander';
import * as fs from 'fs';
import { verifyAuditLog } from '../../audit/AuditVerifier';
import { AuditLogger } from '../../audit/AuditLogger';
import { createAuditSigner } from '../../audit/signing/factory';
import { registerAuditCommands, runAuditSign } from '../audit';

jest.mock('../../audit/AuditVerifier');
jest.mock('fs');
jest.mock('../../audit/signing/factory', () => ({
  createAuditSigner: jest.fn(),
}));
jest.mock('../../audit/AuditLogger', () => ({
  AuditLogger: jest.fn(),
}));

describe('Audit Commands CLI', () => {
  let program: Command;

  beforeEach(() => {
    program = new Command();
    registerAuditCommands(program);
    jest.clearAllMocks();
  });

  describe('audit:verify', () => {
    it('should verify an audit log from a file', async () => {
      const mockLog = { trace: { foo: 'bar' }, signature: 'abc', publicKey: 'pub', hash: '123' };
      (fs.readFileSync as jest.Mock).mockReturnValue(JSON.stringify(mockLog));
      (verifyAuditLog as jest.Mock).mockReturnValue(true);

      const consoleLogSpy = jest.spyOn(console, 'log').mockImplementation();

      await program.parseAsync(['node', 'test', 'audit:verify', '--file', 'test.json']);

      expect(fs.readFileSync).toHaveBeenCalledWith('test.json', 'utf8');
      expect(verifyAuditLog).toHaveBeenCalledWith(mockLog);
      expect(consoleLogSpy).toHaveBeenCalledWith(expect.stringContaining('[OK] Verification successful'));

      consoleLogSpy.mockRestore();
    });

    it('should verify from individual components', async () => {
      const payload = JSON.stringify({ amount: 100 });
      const sig = 'deadbeef';
      const pubkey = 'pem-content';

      (verifyAuditLog as jest.Mock).mockReturnValue(true);
      const consoleLogSpy = jest.spyOn(console, 'log').mockImplementation();

      await program.parseAsync([
        'node',
        'test',
        'audit:verify',
        '--payload',
        payload,
        '--sig',
        sig,
        '--pubkey',
        pubkey,
      ]);

      expect(verifyAuditLog).toHaveBeenCalledWith(
        expect.objectContaining({
          trace: { amount: 100 },
          signature: sig,
          publicKey: pubkey,
        })
      );
      expect(consoleLogSpy).toHaveBeenCalledWith(expect.stringContaining('[OK] Verification successful'));

      consoleLogSpy.mockRestore();
    });

    it('should fail if signature is invalid', async () => {
      const mockLog = { trace: { foo: 'bar' }, signature: 'bad', publicKey: 'pub', hash: '123' };
      (fs.readFileSync as jest.Mock).mockReturnValue(JSON.stringify(mockLog));
      (verifyAuditLog as jest.Mock).mockReturnValue(false);

      const consoleErrorSpy = jest.spyOn(console, 'error').mockImplementation();
      const processExitSpy = jest.spyOn(process, 'exit').mockImplementation((() => {
        return undefined as never;
      }) as any);

      await program.parseAsync(['node', 'test', 'audit:verify', '--file', 'test.json']);

      expect(verifyAuditLog).toHaveBeenCalled();
      expect(consoleErrorSpy).toHaveBeenCalledWith(expect.stringContaining('[FAIL] Verification failed'));
      expect(processExitSpy).toHaveBeenCalledWith(1);

      consoleErrorSpy.mockRestore();
      processExitSpy.mockRestore();
    });

    it('should throw error if missing arguments', async () => {
      const consoleErrorSpy = jest.spyOn(console, 'error').mockImplementation();
      const processExitSpy = jest.spyOn(process, 'exit').mockImplementation((() => {
        return undefined as never;
      }) as any);

      await program.parseAsync(['node', 'test', 'audit:verify', '--payload', '{}']);

      expect(consoleErrorSpy).toHaveBeenCalledWith(expect.stringContaining('You must provide either --file or all of'));
      expect(processExitSpy).toHaveBeenCalledWith(1);

      consoleErrorSpy.mockRestore();
      processExitSpy.mockRestore();
    });
  });
});

describe('runAuditSign', () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  it('should run dry-run validations without generating a signature', async () => {
    const signer = {
      preflight: jest.fn().mockResolvedValue(undefined),
      public_key: jest.fn().mockResolvedValue('-----BEGIN PUBLIC KEY-----\nmock\n-----END PUBLIC KEY-----'),
      sign: jest.fn(),
    };

    (createAuditSigner as jest.Mock).mockReturnValue(signer);

    const stdout = { write: jest.fn() };
    const stderr = { write: jest.fn() };
    const exit = jest.fn(() => {
      throw new Error('exit should not be called');
    }) as never;

    await runAuditSign(
      {
        payload: '{"state":{"b":2,"a":1}}',
        dryRun: true,
        hsmProvider: 'pkcs11',
      },
      { stdout, stderr, exit }
    );

    expect(createAuditSigner).toHaveBeenCalledTimes(1);
    expect(signer.preflight).toHaveBeenCalledTimes(1);
    expect(signer.public_key).toHaveBeenCalledTimes(1);
    expect(AuditLogger).not.toHaveBeenCalled();
    expect(stderr.write).not.toHaveBeenCalled();

    const payload = JSON.parse((stdout.write as jest.Mock).mock.calls[0][0]);
    expect(payload.dryRun).toBe(true);
    expect(payload.status).toBe('ok');
    expect(payload.validations).toEqual({
      payloadParsed: true,
      canonicalized: true,
      connectivityChecked: true,
    });
    expect(payload.signer.provider).toBe('pkcs11');
    expect(payload.traceHash).toMatch(/^[a-f0-9]{64}$/);
  });

  it('should generate a signed audit log when dry-run is disabled', async () => {
    const signer = {
      public_key: jest.fn().mockResolvedValue('mock-public-key'),
      sign: jest.fn(),
    };
    const expectedLog = { hash: 'abc123', signature: 'deadbeef' };

    (createAuditSigner as jest.Mock).mockReturnValue(signer);
    (AuditLogger as jest.Mock).mockImplementation(() => ({
      generateLog: jest.fn().mockResolvedValue(expectedLog),
    }));

    const stdout = { write: jest.fn() };
    const stderr = { write: jest.fn() };
    const exit = jest.fn(() => {
      throw new Error('exit should not be called');
    }) as never;

    await runAuditSign(
      {
        payload: '{"input":{"amount":100}}',
      },
      { stdout, stderr, exit }
    );

    expect(AuditLogger).toHaveBeenCalledTimes(1);
    const payload = JSON.parse((stdout.write as jest.Mock).mock.calls[0][0]);
    expect(payload).toEqual(expectedLog);
  });
});
