// Copyright (c) 2026 dotandev
// SPDX-License-Identifier: MIT OR Apache-2.0

import { AuditLogger } from '../../audit/AuditLogger';
import { createAuditSigner } from '../../audit/signing/factory';
import { runAuditSign } from '../audit';

jest.mock('../../audit/signing/factory', () => ({
    createAuditSigner: jest.fn(),
}));

jest.mock('../../audit/AuditLogger', () => ({
    AuditLogger: jest.fn(),
}));

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
