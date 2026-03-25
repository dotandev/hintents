// Copyright (c) Hintents Authors.
// SPDX-License-Identifier: Apache-2.0

import {
  ContractDeployment,
  DeploymentManifest,
  DeploymentPlan,
  DeploymentResult,
  DeployedContract,
} from './types';
import { createDeploymentPlan, topologicalSort } from './dag';

const CONTRACT_REF_PATTERN = /\$CONTRACT_([A-Za-z_][A-Za-z0-9_]*)/g;

export class DeploymentResolver {
  private deployedContracts: Map<string, DeployedContract> = new Map();

  async deploy(
    manifest: DeploymentManifest,
    deployer: (contract: ContractDeployment, initArgs: string[]) => Promise<DeployedContract>
  ): Promise<DeploymentResult[]> {
    const plan = createDeploymentPlan(manifest.contracts);
    const results: DeploymentResult[] = [];

    for (const contract of plan.order) {
      try {
        const resolvedArgs = this.resolveReferences(contract.initArgs || [], plan.resolved);

        const deployed = await deployer(contract, resolvedArgs);

        this.deployedContracts.set(contract.name, deployed);
        plan.resolved.set(contract.name, deployed.id);

        results.push({
          contractName: contract.name,
          contractId: deployed.id,
          success: true,
        });
      } catch (error) {
        results.push({
          contractName: contract.name,
          contractId: '',
          success: false,
          error: error instanceof Error ? error.message : String(error),
        });

        return results;
      }
    }

    return results;
  }

  resolveReferences(args: string[], resolved: Map<string, string>): string[] {
    return args.map((arg) => {
      return arg.replace(CONTRACT_REF_PATTERN, (_, name) => {
        const id = resolved.get(name);
        if (!id) {
          throw new Error(`Unresolved contract reference: $CONTRACT_${name}`);
        }
        return id;
      });
    });
  }

  getDeployedContract(name: string): DeployedContract | undefined {
    return this.deployedContracts.get(name);
  }

  getAllDeployed(): Map<string, DeployedContract> {
    return new Map(this.deployedContracts);
  }
}

export function parseManifest(content: string): DeploymentManifest {
  try {
    return JSON.parse(content);
  } catch {
    throw new Error('Failed to parse manifest: invalid JSON');
  }
}

export function validateManifest(manifest: DeploymentManifest): string[] {
  const errors: string[] = [];
  const contractNames = new Set<string>();

  if (!manifest.version) {
    errors.push('Missing required field: version');
  }

  if (!manifest.network) {
    errors.push('Missing required field: network');
  }

  if (!manifest.contracts || !Array.isArray(manifest.contracts)) {
    errors.push('Missing or invalid field: contracts must be an array');
    return errors;
  }

  for (const contract of manifest.contracts) {
    if (!contract.name) {
      errors.push('Contract missing required field: name');
    } else {
      if (contractNames.has(contract.name)) {
        errors.push(`Duplicate contract name: ${contract.name}`);
      }
      contractNames.add(contract.name);
    }

    if (!contract.wasm) {
      errors.push(`Contract "${contract.name || 'unknown'}" missing required field: wasm`);
    }

    if (contract.dependencies) {
      for (const dep of contract.dependencies) {
        if (!contractNames.has(dep) && !manifest.contracts.some((c) => c.name === dep)) {
          errors.push(
            `Contract "${contract.name}" depends on undefined contract: ${dep}`
          );
        }
      }
    }
  }

  return errors;
}
