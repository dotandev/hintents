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

/**
 * Extract one concrete cycle from the set of nodes Kahn's algorithm left
 * un-sorted. These are exactly the nodes still trapped in a cycle (or
 * downstream of one).
 *
 * Walks `dependencies` edges (contract to its deps) staying inside `remaining`
 * until a node repeats; the segment from the first occurrence to the repeat
 * is the cycle. Returns `[A, B, C, A]` for the trace `A -> B -> C -> A`.
 */
function findCycle(
  remaining: Set<string>,
  contractMap: Map<string, ContractDeployment>
): string[] {
  for (const start of remaining) {
    const path: string[] = [];
    const seenIndex = new Map<string, number>();
    let current: string | undefined = start;

    while (current !== undefined) {
      const cycleStart = seenIndex.get(current);
      if (cycleStart !== undefined) {
        // Found the loop; close it by re-appending `current`.
        return [...path.slice(cycleStart), current];
      }
      seenIndex.set(current, path.length);
      path.push(current);

      const contract = contractMap.get(current);
      const deps = contract?.dependencies ?? [];
      current = deps.find((d) => remaining.has(d));
    }
  }

  // Should not be reachable if Kahn's reported a cycle, but keep the
  // fallback safe so the message is still useful.
  return Array.from(remaining);
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
    const sortedNames = new Set(result.map((r) => r.name));
    const remaining = new Set(
      contracts.map((c) => c.name).filter((name) => !sortedNames.has(name))
    );
    const cycle = findCycle(remaining, contractMap);
    const trace = cycle.join(' -> ');
    throw new CycleError(
      `Circular dependency detected in deployment manifest:\n` +
        `  ${trace}\n` +
        `Deployment cannot proceed until this cycle is broken.`
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
