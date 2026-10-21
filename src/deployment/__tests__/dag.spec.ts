// Copyright (c) Hintents Authors.
// SPDX-License-Identifier: Apache-2.0

import { topologicalSort, CycleError } from '../dag';
import { ContractDeployment } from '../types';

describe('DAG - Cycle trace formatting (#2111)', () => {
  it('formats a two-node cycle as A -> B -> A', () => {
    const contracts: ContractDeployment[] = [
      { name: 'A', wasm: 'a.wasm', dependencies: ['B'] },
      { name: 'B', wasm: 'b.wasm', dependencies: ['A'] },
    ];

    try {
      topologicalSort(contracts);
      fail('expected CycleError');
    } catch (err) {
      expect(err).toBeInstanceOf(CycleError);
      const message = (err as Error).message;
      const okA = message.includes('A -> B -> A');
      const okB = message.includes('B -> A -> B');
      expect(okA || okB).toBe(true);
      expect(message).toContain('Deployment cannot proceed');
    }
  });

  it('formats a three-node cycle as a closed path', () => {
    const contracts: ContractDeployment[] = [
      { name: 'A', wasm: 'a.wasm', dependencies: ['B'] },
      { name: 'B', wasm: 'b.wasm', dependencies: ['C'] },
      { name: 'C', wasm: 'c.wasm', dependencies: ['A'] },
    ];

    try {
      topologicalSort(contracts);
      fail('expected CycleError');
    } catch (err) {
      expect(err).toBeInstanceOf(CycleError);
      const message = (err as Error).message;
      const rotations = [
        'A -> B -> C -> A',
        'B -> C -> A -> B',
        'C -> A -> B -> C',
      ];
      expect(rotations.some((r) => message.includes(r))).toBe(true);
    }
  });

  it('detects a self-loop as A -> A', () => {
    const contracts: ContractDeployment[] = [
      { name: 'A', wasm: 'a.wasm', dependencies: ['A'] },
    ];

    try {
      topologicalSort(contracts);
      fail('expected CycleError');
    } catch (err) {
      expect(err).toBeInstanceOf(CycleError);
      expect((err as Error).message).toContain('A -> A');
    }
  });

  it('does not claim nodes upstream of a cycle are part of it', () => {
    const contracts: ContractDeployment[] = [
      { name: 'X', wasm: 'x.wasm' },
      { name: 'A', wasm: 'a.wasm', dependencies: ['B'] },
      { name: 'B', wasm: 'b.wasm', dependencies: ['A'] },
    ];

    try {
      topologicalSort(contracts);
      fail('expected CycleError');
    } catch (err) {
      expect(err).toBeInstanceOf(CycleError);
      const message = (err as Error).message;
      const okA = message.includes('A -> B -> A');
      const okB = message.includes('B -> A -> B');
      expect(okA || okB).toBe(true);
      expect(message).not.toContain('X -> A');
    }
  });

  it('reports a valid trace when the manifest has two independent cycles', () => {
    const contracts: ContractDeployment[] = [
      { name: 'A', wasm: 'a.wasm', dependencies: ['B'] },
      { name: 'B', wasm: 'b.wasm', dependencies: ['A'] },
      { name: 'C', wasm: 'c.wasm', dependencies: ['D'] },
      { name: 'D', wasm: 'd.wasm', dependencies: ['C'] },
    ];

    try {
      topologicalSort(contracts);
      fail('expected CycleError');
    } catch (err) {
      expect(err).toBeInstanceOf(CycleError);
      const message = (err as Error).message;
      const ab = message.includes('A -> B -> A') || message.includes('B -> A -> B');
      const cd = message.includes('C -> D -> C') || message.includes('D -> C -> D');
      expect(ab || cd).toBe(true);
    }
  });

  it('does not throw on a linear chain (regression)', () => {
    const contracts: ContractDeployment[] = [
      { name: 'A', wasm: 'a.wasm' },
      { name: 'B', wasm: 'b.wasm', dependencies: ['A'] },
      { name: 'C', wasm: 'c.wasm', dependencies: ['B'] },
    ];

    const result = topologicalSort(contracts);
    expect(result.map((c) => c.name)).toEqual(['A', 'B', 'C']);
  });

  it('does not throw on a diamond (regression)', () => {
    const contracts: ContractDeployment[] = [
      { name: 'A', wasm: 'a.wasm' },
      { name: 'B', wasm: 'b.wasm', dependencies: ['A'] },
      { name: 'C', wasm: 'c.wasm', dependencies: ['A'] },
      { name: 'D', wasm: 'd.wasm', dependencies: ['B', 'C'] },
    ];

    expect(() => topologicalSort(contracts)).not.toThrow();
  });
});
