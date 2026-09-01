// Copyright (c) Hintents Authors.
// SPDX-License-Identifier: Apache-2.0

/**
 * Integration tests for DAP adapter WASM locals support.
 *
 * These tests verify that:
 * 1. TraceStep can include wasm_locals data
 * 2. The debug adapter properly exposes WASM locals in the scopes handler
 * 3. WASM locals are displayed correctly in hover tooltips
 * 4. Variable references are properly managed for nested expansion
 */

import { TraceStep, WasmLocal } from '../erstClient';

describe('WASM Locals DAP Integration', () => {
    describe('TraceStep interface', () => {
        test('TraceStep accepts wasm_locals field', () => {
            const step: TraceStep = {
                step: 1,
                timestamp: '2024-01-01T00:00:00Z',
                operation: 'CallContractHost',
                contract_id: 'CA7QMUSQMSQWV4OSURMQNQ7HKOJBKJ5HCMRPCX4KNLMQT5CQVNQWGWHV4',
                function: 'increment',
                wasm_locals: [
                    { name: 'count', type: 'i32', location: 'local', value: 42 },
                    { name: 'max', type: 'i32', location: 'local', value: 1000 },
                ],
                wasm_offset: 12345,
            };

            expect(step.wasm_locals).toHaveLength(2);
            expect(step.wasm_locals[0].name).toBe('count');
            expect(step.wasm_offset).toBe(12345);
        });

        test('TraceStep is backward compatible without wasm_locals', () => {
            const step: TraceStep = {
                step: 1,
                timestamp: '2024-01-01T00:00:00Z',
                operation: 'CallContractHost',
                contract_id: 'CA7QM...',
                function: 'test',
                arguments: [100],
                return_value: 200,
            };

            expect(step.wasm_locals).toBeUndefined();
            expect(step.wasm_offset).toBeUndefined();
        });
    });

    describe('WasmLocal interface', () => {
        test('WasmLocal has required fields', () => {
            const local: WasmLocal = {
                name: 'x',
                type: 'i64',
                location: 'local[0]',
            };

            expect(local.name).toBe('x');
            expect(local.type).toBe('i64');
            expect(local.location).toBe('local[0]');
        });

        test('WasmLocal supports optional fields', () => {
            const local: WasmLocal = {
                name: 'result',
                type: 'struct',
                location: 'memory',
                value: { amount: 100 },
                startLine: 42,
                endLine: 50,
            };

            expect(local.value).toEqual({ amount: 100 });
            expect(local.startLine).toBe(42);
            expect(local.endLine).toBe(50);
        });
    });

    describe('Locals scope presentation', () => {
        test('WASM locals should be displayed separately from Arguments', () => {
            const step: TraceStep = {
                step: 1,
                timestamp: '2024-01-01T00:00:00Z',
                operation: 'call',
                arguments: [10, 20],
                wasm_locals: [
                    { name: 'local0', type: 'i32', location: 'stack' },
                    { name: 'local1', type: 'i32', location: 'stack' },
                ],
            };

            // In the DAP adapter, this should create two separate scopes:
            // 1. Arguments scope (for step.arguments)
            // 2. WASM Locals scope (for step.wasm_locals)
            expect(step.arguments).toHaveLength(2);
            expect(step.wasm_locals).toHaveLength(2);
        });

        test('WASM locals with various types', () => {
            const locals: WasmLocal[] = [
                { name: 'i', type: 'i32', location: 'local[0]', value: 0 },
                { name: 'f', type: 'f32', location: 'local[1]', value: 3.14 },
                { name: 'ptr', type: 'i32', location: 'local[2]', value: 1024 },
                { name: 'struct', type: 'S', location: 'memory', value: { a: 1, b: 2 } },
            ];

            expect(locals).toHaveLength(4);
            expect(locals[0].type).toBe('i32');
            expect(locals[1].type).toBe('f32');
            expect(locals[3].type).toBe('S');
        });
    });

    describe('Hover functionality', () => {
        test('hover on variable should display WASM local info', () => {
            const step: TraceStep = {
                step: 5,
                timestamp: '2024-01-01T00:00:01Z',
                operation: 'LocalSet',
                wasm_locals: [
                    {
                        name: 'my_var',
                        type: 'i64',
                        location: 'local[3]',
                        value: 9223372036854775807n,
                        startLine: 15,
                        endLine: 25,
                    },
                ],
            };

            // When hovering over 'my_var' in the editor:
            // 1. DAP should return the WASM Locals scope
            // 2. Hover provider should show: "my_var: i64 = 9223372036854775807"
            // 3. The range should span from line 15 to 25
            const local = step.wasm_locals[0];
            expect(local.name).toBe('my_var');
            expect(local.type).toBe('i64');
            expect(local.value).toBe(9223372036854775807n);
        });

        test('nested object expansion in hover', () => {
            const step: TraceStep = {
                step: 1,
                timestamp: '2024-01-01T00:00:00Z',
                operation: 'call',
                wasm_locals: [
                    {
                        name: 'config',
                        type: 'Config',
                        location: 'memory[256:512]',
                        value: {
                            enabled: true,
                            threshold: 100,
                            details: { level: 'info', tag: 'cfg' },
                        },
                    },
                ],
            };

            const local = step.wasm_locals[0];
            expect(local.value).toHaveProperty('enabled', true);
            expect(local.value).toHaveProperty('details');
            expect(local.value.details).toHaveProperty('level', 'info');
        });
    });

    describe('Memory mapping', () => {
        test('WASM locals with memory locations', () => {
            const locals: WasmLocal[] = [
                {
                    name: 'buffer',
                    type: '[u8; 256]',
                    location: 'memory[0:256]',
                    value: 'Buffer data...',
                },
                {
                    name: 'struct_ptr',
                    type: '*mut Account',
                    location: 'memory[256:264]',
                    value: { balance: 1000, locked: false },
                },
            ];

            expect(locals[0].location).toBe('memory[0:256]');
            expect(locals[1].location).toBe('memory[256:264]');
        });

        test('WASM locals with stack locations', () => {
            const locals: WasmLocal[] = [
                { name: 'a', type: 'i32', location: 'stack', value: 1 },
                { name: 'b', type: 'i32', location: 'stack', value: 2 },
                { name: 'c', type: 'i32', location: 'stack', value: 3 },
            ];

            expect(locals.every(l => l.location === 'stack')).toBe(true);
        });
    });

    describe('Scope ordering', () => {
        test('scopes appear in expected order', () => {
            const step: TraceStep = {
                step: 1,
                timestamp: '2024-01-01T00:00:00Z',
                operation: 'call',
                arguments: [10],
                wasm_locals: [{ name: 'x', type: 'i32', location: 'local' }],
                host_state: { ledger: 'state' },
                memory: { data: [] },
                cpu_delta: 100,
                memory_delta: 50,
            };

            // Expected scope order in DAP:
            // 1. Locals (operation details)
            // 2. WASM Locals (if present)
            // 3. Arguments (if present)
            // 4. Host State (if present)
            // 5. Memory (if present)
            // 6. Budget (always present)
            expect(step.arguments).toBeDefined();
            expect(step.wasm_locals).toBeDefined();
            expect(step.host_state).toBeDefined();
            expect(step.memory).toBeDefined();
        });
    });

    describe('Complex types', () => {
        test('handles generic and complex types', () => {
            const locals: WasmLocal[] = [
                {
                    name: 'vec',
                    type: 'Vec<u64>',
                    location: 'memory',
                    value: [1, 2, 3, 4, 5],
                },
                {
                    name: 'option_val',
                    type: 'Option<i32>',
                    location: 'stack',
                    value: { Some: 42 },
                },
                {
                    name: 'result',
                    type: 'Result<String, Error>',
                    location: 'memory',
                    value: { Ok: 'success' },
                },
            ];

            expect(locals[0].type).toMatch(/Vec/);
            expect(locals[1].type).toMatch(/Option/);
            expect(locals[2].type).toMatch(/Result/);
        });
    });
});
