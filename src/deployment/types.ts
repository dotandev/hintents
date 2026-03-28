// Copyright (c) Hintents Authors.
// SPDX-License-Identifier: Apache-2.0

export interface ContractDependency {
  name: string;
  version?: string;
}

export interface ContractDeployment {
  name: string;
  wasm: string;
  salt?: string;
  initArgs?: string[];
  dependencies?: string[];
}

export interface DeploymentManifest {
  version: string;
  network: string;
  contracts: ContractDeployment[];
}

export interface DeployedContract {
  name: string;
  id: string;
  address: string;
}

export interface DeploymentResult {
  contractName: string;
  contractId: string;
  success: boolean;
  error?: string;
}

export interface DeploymentPlan {
  order: ContractDeployment[];
  resolved: Map<string, string>;
}
