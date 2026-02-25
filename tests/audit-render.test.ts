// Copyright (c) 2026 dotandev
// SPDX-License-Identifier: MIT OR Apache-2.0

import { renderAuditHTML } from '../src/audit/AuditRenderer';
import { AuditLogger } from '../src/audit/AuditLogger';
import { MockAuditSigner } from '../src/audit/signing/mockSigner';
import type { ExecutionTrace, SignedAuditLog } from '../src/audit/AuditLogger';

const baseTrace: ExecutionTrace = {
  input: { amount: 100, currency: 'USD', user_id: 'u_123' },
  state: { balance_before: 500, balance_after: 400 },
  events: ['INIT_TRANSFER', 'DEBIT_ACCOUNT', 'FEE_CALC'],
  timestamp: '2026-02-24T12:00:00.000Z',
};

describe('renderAuditHTML (ExecutionTrace)', () => {
  test('returns a string containing the DOCTYPE declaration', () => {
    const html = renderAuditHTML(baseTrace);
    expect(html).toContain('<!DOCTYPE html>');
  });

  test('uses the provided title in the document', () => {
    const html = renderAuditHTML(baseTrace, 'My Custom Report');
    expect(html).toContain('My Custom Report');
  });

  test('falls back to "Audit Report" when no title is supplied', () => {
    const html = renderAuditHTML(baseTrace);
    expect(html).toContain('Audit Report');
  });

  test('renders input key-value pairs', () => {
    const html = renderAuditHTML(baseTrace);
    expect(html).toContain('amount');
    expect(html).toContain('100');
    expect(html).toContain('user_id');
    expect(html).toContain('u_123');
  });

  test('renders state key-value pairs', () => {
    const html = renderAuditHTML(baseTrace);
    expect(html).toContain('balance_before');
    expect(html).toContain('500');
    expect(html).toContain('balance_after');
    expect(html).toContain('400');
  });

  test('renders all events', () => {
    const html = renderAuditHTML(baseTrace);
    expect(html).toContain('INIT_TRANSFER');
    expect(html).toContain('DEBIT_ACCOUNT');
    expect(html).toContain('FEE_CALC');
  });

  test('shows event count in section heading', () => {
    const html = renderAuditHTML(baseTrace);
    expect(html).toContain('Events (3)');
  });

  test('does not include a signature section for raw traces', () => {
    const html = renderAuditHTML(baseTrace);
    expect(html).not.toContain('id="signature"');
  });

  test('includes timestamp in the generated-at line', () => {
    const html = renderAuditHTML(baseTrace);
    // The timestamp is formatted via Date.toUTCString, so just confirm it's present
    expect(html).toContain('Generated:');
  });

  test('handles an empty events array gracefully', () => {
    const trace: ExecutionTrace = { ...baseTrace, events: [] };
    const html = renderAuditHTML(trace);
    expect(html).toContain('No events recorded.');
    expect(html).toContain('Events (0)');
  });

  test('handles empty input gracefully', () => {
    const trace: ExecutionTrace = { ...baseTrace, input: {} };
    const html = renderAuditHTML(trace);
    expect(html).toContain('No input data.');
  });

  test('handles empty state gracefully', () => {
    const trace: ExecutionTrace = { ...baseTrace, state: {} };
    const html = renderAuditHTML(trace);
    expect(html).toContain('No state data.');
  });

  test('escapes HTML special characters in input values', () => {
    const trace: ExecutionTrace = {
      ...baseTrace,
      input: { note: '<script>alert(1)</script>' },
    };
    const html = renderAuditHTML(trace);
    expect(html).not.toContain('<script>');
    expect(html).toContain('&lt;script&gt;');
  });
});

describe('renderAuditHTML (SignedAuditLog)', () => {
  let signedLog: SignedAuditLog;

  beforeAll(async () => {
    const signer = new MockAuditSigner();
    const logger = new AuditLogger(signer, 'mock');
    signedLog = await logger.generateLog(baseTrace);
  });

  test('includes the signature section', () => {
    const html = renderAuditHTML(signedLog);
    expect(html).toContain('id="signature"');
  });

  test('displays the algorithm', () => {
    const html = renderAuditHTML(signedLog);
    expect(html).toContain('Ed25519+SHA256');
  });

  test('displays the hash', () => {
    const html = renderAuditHTML(signedLog);
    expect(html).toContain(signedLog.hash);
  });

  test('displays the signer provider', () => {
    const html = renderAuditHTML(signedLog);
    expect(html).toContain('mock');
  });

  test('includes signature navigation link in TOC', () => {
    const html = renderAuditHTML(signedLog);
    expect(html).toContain('href="#signature"');
  });

  test('renders input and state from the embedded trace', () => {
    const html = renderAuditHTML(signedLog);
    expect(html).toContain('amount');
    expect(html).toContain('balance_before');
  });

  test('renders all events from the embedded trace', () => {
    const html = renderAuditHTML(signedLog);
    expect(html).toContain('INIT_TRANSFER');
    expect(html).toContain('DEBIT_ACCOUNT');
    expect(html).toContain('FEE_CALC');
  });
});
