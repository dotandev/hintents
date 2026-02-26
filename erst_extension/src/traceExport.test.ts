import test from 'node:test';
import assert from 'node:assert/strict';
import { buildTraceTreeExport, renderStandaloneHtml } from './traceExport';
import { Trace } from './erstClient';

test('buildTraceTreeExport includes matches and expanded nodes', () => {
    const trace: Trace = {
        transaction_hash: 'tx-123',
        start_time: '2026-02-23T10:00:00Z',
        states: [
            {
                step: 1,
                timestamp: '2026-02-23T10:00:01Z',
                operation: 'invoke',
                contract_id: 'CABC123',
                function: 'transfer',
                arguments: ['alice', 'bob', 5],
                return_value: { ok: true }
            },
            {
                step: 2,
                timestamp: '2026-02-23T10:00:02Z',
                operation: 'event',
                error: 'insufficient balance'
            }
        ]
    };

    const payload = buildTraceTreeExport(trace, 'transfer');

    assert.equal(payload.transactionHash, 'tx-123');
    assert.equal(payload.searchQuery, 'transfer');
    assert.ok(payload.totalMatches > 0);
    assert.equal(payload.tree.length, 2);
    assert.ok(payload.tree[0].children.length >= 5);
});

test('renderStandaloneHtml renders metadata and payload json', () => {
    const trace: Trace = {
        transaction_hash: 'tx-html',
        start_time: '2026-02-23T10:00:00Z',
        states: [
            {
                step: 1,
                timestamp: '2026-02-23T10:00:01Z',
                operation: 'invoke'
            }
        ]
    };

    const payload = buildTraceTreeExport(trace, '');
    const html = renderStandaloneHtml(payload);

    assert.ok(html.includes('<!doctype html>'));
    assert.ok(html.includes('ERST Trace Tree Export'));
    assert.ok(html.includes('tx-html'));
    assert.ok(html.includes('&quot;transactionHash&quot;: &quot;tx-html&quot;'));
});
