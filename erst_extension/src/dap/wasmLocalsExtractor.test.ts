// Copyright (c) Hintents Authors.
// SPDX-License-Identifier: Apache-2.0

import { WasmLocal, WasmLocalsExtractor } from './wasmLocalsExtractor';

describe('WasmLocalsExtractor', () => {
    let extractor: WasmLocalsExtractor;

    beforeEach(() => {
        extractor = new WasmLocalsExtractor();
    });

    describe('initialization', () => {
        test('creates extractor without WASM data', () => {
            expect(extractor).toBeDefined();
            expect(extractor.hasDebugInfo()).toBe(false);
        });

        test('creates extractor with WASM data', () => {
            const wasmBytes = Buffer.from([0x00, 0x61, 0x73, 0x6d]); // WASM magic
            const extractorWithData = new WasmLocalsExtractor(wasmBytes);
            expect(extractorWithData).toBeDefined();
            expect(extractorWithData.hasDebugInfo()).toBe(false); // No debug symbols in minimal WASM
        });
    });

    describe('debug symbol detection', () => {
        test('detects non-WASM binary', () => {
            const nonWasmData = Buffer.from('not a wasm binary');
            extractor.setWasmData(nonWasmData);
            expect(extractor.hasDebugInfo()).toBe(false);
        });

        test('detects WASM magic number', () => {
            const wasmMagic = Buffer.from([0x00, 0x61, 0x73, 0x6d]);
            extractor.setWasmData(wasmMagic);
            expect(extractor.hasDebugInfo()).toBe(false); // No debug sections
        });
    });

    describe('locals extraction', () => {
        test('extracts locals from step data with wasm_locals field', () => {
            const stepData = {
                step: 1,
                operation: 'call',
                wasm_locals: [
                    { name: 'local0', type: 'i32', location: 'stack', value: 42 },
                    { name: 'local1', type: 'i64', location: 'stack', value: 123n },
                ],
            };

            const locals = extractor.extractLocalsFromStepData(stepData);
            expect(locals).toHaveLength(2);
            expect(locals[0]).toEqual({
                name: 'local0',
                type: 'i32',
                location: 'stack',
                value: 42,
                startLine: undefined,
                endLine: undefined,
            });
        });

        test('extracts locals from arguments if wasm_locals not present', () => {
            const stepData = {
                step: 1,
                operation: 'call',
                arguments: [100, 200, 300],
            };

            const locals = extractor.extractLocalsFromStepData(stepData);
            expect(locals).toHaveLength(3);
            expect(locals[0]).toEqual({
                name: 'arg0',
                type: 'any',
                location: 'argument 0',
                value: 100,
            });
            expect(locals[1]).toEqual({
                name: 'arg1',
                type: 'any',
                location: 'argument 1',
                value: 200,
            });
        });

        test('returns empty array for null or invalid data', () => {
            expect(extractor.extractLocalsFromStepData(null)).toEqual([]);
            expect(extractor.extractLocalsFromStepData(undefined)).toEqual([]);
            expect(extractor.extractLocalsFromStepData('invalid')).toEqual([]);
        });

        test('returns empty array for objects without locals or arguments', () => {
            const stepData = { step: 1, operation: 'call' };
            expect(extractor.extractLocalsFromStepData(stepData)).toEqual([]);
        });
    });

    describe('caching', () => {
        test('caches extracted locals by instruction', () => {
            const locals: WasmLocal[] = [
                { name: 'x', type: 'i32', location: 'local', value: 5 },
            ];
            extractor.setWasmData(Buffer.from([0x00, 0x61, 0x73, 0x6d]));

            const result1 = extractor.extractLocalsAtInstruction(100);
            const result2 = extractor.extractLocalsAtInstruction(100);

            expect(result1.instruction).toBe(100);
            expect(result2.instruction).toBe(100);
        });

        test('clears cache on setWasmData', () => {
            extractor.setWasmData(Buffer.from([0x00, 0x61, 0x73, 0x6d]));
            extractor.extractLocalsAtInstruction(100);

            extractor.setWasmData(Buffer.from([0x00, 0x61, 0x73, 0x6d]));
            const result = extractor.extractLocalsAtInstruction(100);

            expect(result.locals).toEqual([]);
        });

        test('clearCache method empties the cache', () => {
            extractor.clearCache();
            expect(extractor.extractLocalsAtInstruction(100).locals).toEqual([]);
        });
    });

    describe('scope extraction', () => {
        test('extracts scope with contract ID and function name', () => {
            const stepData = {
                wasm_locals: [
                    { name: 'i', type: 'i32', location: 'local', value: 0 },
                ],
            };

            extractor.extractLocalsFromStepData(stepData);
            const scope = extractor.extractLocalsAtInstruction(
                50,
                'CA7QMUSQMSQWV4OSURMQNQ7HKOJBKJ5HCMRPCX4KNLMQT5CQVNQWGWHV4',
                'increase_counter'
            );

            expect(scope.instruction).toBe(50);
            expect(scope.contractId).toBe('CA7QMUSQMSQWV4OSURMQNQ7HKOJBKJ5HCMRPCX4KNLMQT5CQVNQWGWHV4');
            expect(scope.functionName).toBe('increase_counter');
        });
    });

    describe('edge cases', () => {
        test('handles step data with wasm_locals as empty array', () => {
            const stepData = { wasm_locals: [] };
            const locals = extractor.extractLocalsFromStepData(stepData);
            expect(locals).toEqual([]);
        });

        test('handles wasm_locals with missing fields', () => {
            const stepData = {
                wasm_locals: [
                    { name: 'local0' }, // Missing type, location
                ],
            };

            const locals = extractor.extractLocalsFromStepData(stepData);
            expect(locals).toHaveLength(1);
            expect(locals[0].name).toBe('local0');
            expect(locals[0].type).toBe('any');
            expect(locals[0].location).toBe('');
        });

        test('preserves all WasmLocal fields', () => {
            const stepData = {
                wasm_locals: [
                    {
                        name: 'result',
                        type: 'i32',
                        location: 'stack',
                        value: 42,
                        startLine: 10,
                        endLine: 20,
                    },
                ],
            };

            const locals = extractor.extractLocalsFromStepData(stepData);
            expect(locals[0]).toEqual({
                name: 'result',
                type: 'i32',
                location: 'stack',
                value: 42,
                startLine: 10,
                endLine: 20,
            });
        });
    });
});
