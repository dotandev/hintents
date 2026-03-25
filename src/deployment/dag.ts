// Copyright (c) Hintents Authors.
// SPDX-License-Identifier: Apache-2.0

import { ContractDeployment, DeploymentPlan } from './types';

export class CycleError extends Error {
  constructor(message: string) {
    super(message);
    this.name = 'CycleError';
  }
}

export class MissingDependencyError extends Error {
  constructor(message: string) {
    super(message);
    this.name = 'MissingDependencyError';
  }
}

export function topologicalSort(contracts: ContractDeployment[]): ContractDeployment[] {
  const graph = new Map<string, string[]>();
  const inDegree = new Map<string, number>();
  const contractMap = new Map<string, ContractDeployment>();

  for (const contract of contracts) {
    graph.set(contract.name, []);
    inDegree.set(contract.name, 0);
    contractMap.set(contract.name, contract);
  }

  for (const contract of contracts) {
    if (contract.dependencies) {
      for (const dep of contract.dependencies) {
        if (!contractMap.has(dep)) {
          throw new MissingDependencyError(
            `Contract "${contract.name}" depends on "${dep}", which is not defined in the manifest`
          );
        }
        graph.get(dep)!.push(contract.name);
        inDegree.set(contract.name, inDegree.get(contract.name)! + 1);
      }
    }
  }

  const queue: string[] = [];
  Array.from(inDegree.entries()).forEach(([name, degree]) => {
    if (degree === 0) {
      queue.push(name);
    }
  });

  const result: ContractDeployment[] = [];

  while (queue.length > 0) {
    const current = queue.shift()!;
    result.push(contractMap.get(current)!);

    const neighbors = graph.get(current) || [];
    for (const neighbor of neighbors) {
      const newDegree = inDegree.get(neighbor)! - 1;
      inDegree.set(neighbor, newDegree);
      if (newDegree === 0) {
        queue.push(neighbor);
      }
    }
  }

  if (result.length !== contracts.length) {
    const remaining = contracts
      .map((c) => c.name)
      .filter((name) => !result.map((r) => r.name).includes(name));
    throw new CycleError(
      `Circular dependency detected involving contracts: ${remaining.join(', ')}`
    );
  }

  return result;
}

export function createDeploymentPlan(
  contracts: ContractDeployment[]
): DeploymentPlan {
  const sorted = topologicalSort(contracts);
  return {
    order: sorted,
    resolved: new Map(),
  };
}
