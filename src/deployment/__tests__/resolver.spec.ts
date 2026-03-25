// Copyright (c) Hintents Authors.
// SPDX-License-Identifier: Apache-2.0

import {
  topologicalSort,
  createDeploymentPlan,
  CycleError,
  MissingDependencyError,
} from '../dag';
import {
  DeploymentResolver,
  parseManifest,
  validateManifest,
} from '../resolver';
import { ContractDeployment, DeploymentManifest } from '../types';

describe('DAG - Topological Sort', () => {
  describe('topologicalSort', () => {
    it('should sort contracts with no dependencies', () => {
      const contracts: ContractDeployment[] = [
        { name: 'A', wasm: 'a.wasm' },
        { name: 'B', wasm: 'b.wasm' },
        { name: 'C', wasm: 'c.wasm' },
      ];

      const result = topologicalSort(contracts);
      expect(result.map((c) => c.name)).toEqual(['A', 'B', 'C']);
    });

    it('should sort contracts with dependencies correctly', () => {
      const contracts: ContractDeployment[] = [
        { name: 'A', wasm: 'a.wasm', dependencies: [] },
        { name: 'B', wasm: 'b.wasm', dependencies: ['A'] },
        { name: 'C', wasm: 'c.wasm', dependencies: ['B'] },
      ];

      const result = topologicalSort(contracts);
      expect(result.map((c) => c.name)).toEqual(['A', 'B', 'C']);
    });

    it('should handle diamond dependencies', () => {
      const contracts: ContractDeployment[] = [
        { name: 'A', wasm: 'a.wasm' },
        { name: 'B', wasm: 'b.wasm', dependencies: ['A'] },
        { name: 'C', wasm: 'c.wasm', dependencies: ['A'] },
        { name: 'D', wasm: 'd.wasm', dependencies: ['B', 'C'] },
      ];

      const result = topologicalSort(contracts);
      expect(result[0].name).toBe('A');
      expect(result[3].name).toBe('D');
    });

    it('should throw CycleError for circular dependencies', () => {
      const contracts: ContractDeployment[] = [
        { name: 'A', wasm: 'a.wasm', dependencies: ['B'] },
        { name: 'B', wasm: 'b.wasm', dependencies: ['A'] },
      ];

      expect(() => topologicalSort(contracts)).toThrow(CycleError);
    });

    it('should throw MissingDependencyError for undefined dependencies', () => {
      const contracts: ContractDeployment[] = [
        { name: 'A', wasm: 'a.wasm', dependencies: ['B'] },
      ];

      expect(() => topologicalSort(contracts)).toThrow(MissingDependencyError);
    });
  });

  describe('createDeploymentPlan', () => {
    it('should create a valid deployment plan', () => {
      const contracts: ContractDeployment[] = [
        { name: 'Token', wasm: 'token.wasm' },
        { name: 'Vault', wasm: 'vault.wasm', dependencies: ['Token'] },
      ];

      const plan = createDeploymentPlan(contracts);

      expect(plan.order.length).toBe(2);
      expect(plan.order[0].name).toBe('Token');
      expect(plan.resolved.size).toBe(0);
    });
  });
});

describe('DeploymentResolver', () => {
  let resolver: DeploymentResolver;

  beforeEach(() => {
    resolver = new DeploymentResolver();
  });

  describe('resolveReferences', () => {
    it('should replace contract references with IDs', () => {
      const resolved = new Map<string, string>([
        ['Token', 'CA7Y4HPA7M7EJ4AQXTJ2J5QHV5DTB3JENWQ5KVK6M3VY7X4O6FQ2Z5'],
        ['Vault', 'CB7Y4HPA7M7EJ4AQXTJ2J5QHV5DTB3JENWQ5KVK6M3VY7X4O6FQ2Z6'],
      ]);

      const args = ['init', '$CONTRACT_Token', '$CONTRACT_Vault'];
      const result = resolver.resolveReferences(args, resolved);

      expect(result).toEqual([
        'init',
        'CA7Y4HPA7M7EJ4AQXTJ2J5QHV5DTB3JENWQ5KVK6M3VY7X4O6FQ2Z5',
        'CB7Y4HPA7M7EJ4AQXTJ2J5QHV5DTB3JENWQ5KVK6M3VY7X4O6FQ2Z6',
      ]);
    });

    it('should throw error for unresolved references', () => {
      const resolved = new Map<string, string>();
      const args = ['init', '$CONTRACT_Missing'];

      expect(() => resolver.resolveReferences(args, resolved)).toThrow(
        'Unresolved contract reference: $CONTRACT_Missing'
      );
    });

    it('should handle args without references', () => {
      const resolved = new Map<string, string>();
      const args = ['arg1', 'arg2'];
      const result = resolver.resolveReferences(args, resolved);

      expect(result).toEqual(['arg1', 'arg2']);
    });
  });

  describe('deploy', () => {
    it('should deploy contracts in correct order', async () => {
      const manifest: DeploymentManifest = {
        version: '1.0',
        network: 'testnet',
        contracts: [
          { name: 'Token', wasm: 'token.wasm' },
          { name: 'Vault', wasm: 'vault.wasm', dependencies: ['Token'] },
        ],
      };

      const deploymentOrder: string[] = [];

      const mockDeployer = async (
        contract: ContractDeployment,
        _initArgs: string[]
      ) => {
        deploymentOrder.push(contract.name);
        return {
          name: contract.name,
          id: `${contract.name.toUpperCase()}_ID`,
          address: `${contract.name.toUpperCase()}_ADDRESS`,
        };
      };

      const results = await resolver.deploy(manifest, mockDeployer);

      expect(deploymentOrder).toEqual(['Token', 'Vault']);
      expect(results).toHaveLength(2);
      expect(results[0].success).toBe(true);
      expect(results[1].success).toBe(true);
    });

    it('should stop deployment on failure and return error', async () => {
      const manifest: DeploymentManifest = {
        version: '1.0',
        network: 'testnet',
        contracts: [
          { name: 'Token', wasm: 'token.wasm' },
          { name: 'Vault', wasm: 'vault.wasm', dependencies: ['Token'] },
        ],
      };

      const mockDeployer = async (contract: ContractDeployment) => {
        if (contract.name === 'Token') {
          throw new Error('Token deployment failed');
        }
        return {
          name: contract.name,
          id: `${contract.name.toUpperCase()}_ID`,
          address: `${contract.name.toUpperCase()}_ADDRESS`,
        };
      };

      const results = await resolver.deploy(manifest, mockDeployer);

      expect(results).toHaveLength(1);
      expect(results[0].success).toBe(false);
      expect(results[0].error).toBe('Token deployment failed');
    });
  });
});

describe('Manifest parsing and validation', () => {
  describe('parseManifest', () => {
    it('should parse valid JSON manifest', () => {
      const json = JSON.stringify({
        version: '1.0',
        network: 'testnet',
        contracts: [{ name: 'A', wasm: 'a.wasm' }],
      });

      const manifest = parseManifest(json);

      expect(manifest.version).toBe('1.0');
      expect(manifest.network).toBe('testnet');
      expect(manifest.contracts).toHaveLength(1);
    });

    it('should throw error for invalid JSON', () => {
      expect(() => parseManifest('invalid json')).toThrow('Failed to parse manifest');
    });
  });

  describe('validateManifest', () => {
    it('should return no errors for valid manifest', () => {
      const manifest: DeploymentManifest = {
        version: '1.0',
        network: 'testnet',
        contracts: [{ name: 'A', wasm: 'a.wasm' }],
      };

      const errors = validateManifest(manifest);
      expect(errors).toHaveLength(0);
    });

    it('should detect missing required fields', () => {
      const manifest = {
        version: '1.0',
        contracts: [{ name: 'A', wasm: 'a.wasm' }],
      } as DeploymentManifest;

      const errors = validateManifest(manifest);
      expect(errors).toContain('Missing required field: network');
    });

    it('should detect duplicate contract names', () => {
      const manifest: DeploymentManifest = {
        version: '1.0',
        network: 'testnet',
        contracts: [
          { name: 'A', wasm: 'a.wasm' },
          { name: 'A', wasm: 'a2.wasm' },
        ],
      };

      const errors = validateManifest(manifest);
      expect(errors).toContain('Duplicate contract name: A');
    });

    it('should detect undefined dependencies', () => {
      const manifest: DeploymentManifest = {
        version: '1.0',
        network: 'testnet',
        contracts: [{ name: 'A', wasm: 'a.wasm', dependencies: ['B'] }],
      };

      const errors = validateManifest(manifest);
      expect(errors).toContain('Contract "A" depends on undefined contract: B');
    });
  });
});
