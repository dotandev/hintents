// Copyright (c) 2026 dotandev
// SPDX-License-Identifier: MIT OR Apache-2.0

export type RPCMethod =
    | 'getHealth'
    | 'getLatestLedger'
    | 'getTransaction'
    | 'getTransactions'
    | 'getLedgerEntries'
    | 'simulateTransaction'
    | 'sendTransaction'
    | 'getEvents'
    | 'getNetwork'
    | 'getFeeStats'
    | 'getVersionInfo';

export interface RPCMethodParams {
    getHealth?: Record<string, unknown>;
    getLatestLedger?: Record<string, unknown>;
    getTransaction?: { hash: string };
    getTransactions?: { cursor?: string; limit?: number; order?: 'asc' | 'desc' };
    getLedgerEntries?: { keys: string[]; cursor?: string };
    simulateTransaction?: { transaction: string; simulationConfig?: SimulationConfig };
    sendTransaction?: { transaction: string };
    getEvents?: GetEventsParams;
    getNetwork?: Record<string, unknown>;
    getFeeStats?: Record<string, unknown>;
    getVersionInfo?: Record<string, unknown>;
}

export interface GetEventsParams {
    startLedger: number;
    endLedger?: number;
    filters?: EventFilter[];
    limit?: number;
    cursor?: string;
}

export interface EventFilter {
    type?: string[];
    contractIds?: string[];
    topics?: string[];
}

export interface SimulationConfig {
    enableDebug?: boolean;
    maxInstructions?: number;
    cpuFeeRate?: string;
    memFeeRate?: string;
}

export interface RPCRequest<T extends RPCMethod = RPCMethod> {
    jsonrpc: '2.0';
    id: number | string;
    method: T;
    params?: RPCMethodParams[T];
}

export interface RPCResponse<T = unknown> {
    jsonrpc: '2.0';
    id: number | string;
    result?: T;
    error?: RPCError;
}

export interface RPCError {
    code: number;
    message: string;
    data?: unknown;
}

export interface BatchRequestItem {
    id: number | string;
    method: RPCMethod;
    params?: RPCMethodParams[RPCMethod];
}

export interface BatchResponseItem {
    id: number | string;
    success?: boolean;
    result?: unknown;
    error?: RPCError;
}

export interface BatchRequest<T extends RPCMethod = RPCMethod> {
    jsonrpc: '2.0';
    id: number | string;
    method: 'batch';
    params: {
        requests: Array<RPCRequest<T>>;
    };
}

export interface BatchResponse {
    jsonrpc: '2.0';
    id: number | string;
    result?: BatchResponseItem[];
    error?: RPCError;
}

export interface HealthResponse {
    status: 'healthy' | 'unhealthy';
    currentProtocolVersion?: string;
    currentLedgerVersion?: number;
    buildVersion?: string;
    networkPassphrase?: string;
    currentProtocolVersionIface?: number;
}

export interface TransactionResponse {
    hash: string;
    ledger: number;
    createdAt: string;
    feeAccount: string;
    successful: boolean;
    operationResults?: unknown[];
    transactions?: unknown[];
    memo?: string;
    feeCharged?: string;
    minFee?: string;
    maxFee?: string;
}

export interface SimulateTransactionResponse {
    results?: Array<{
        footprint?: string;
        auth?: string[];
        events?: string[];
        returnValue?: string;
        diagnostics?: unknown[];
        cost?: {
            cpuInstructions: string;
            memoryBytes: string;
        };
    }>;
    cost?: {
        cpuInstructions: string;
        memoryBytes: string;
    };
    latestLedger?: number;
    latestLedgerCloseTime?: number;
}

export interface SendTransactionResponse {
    hash: string;
    status: 'pending' | 'success' | 'failed';
    ledger?: number;
    createdAt?: string;
    feeAccount?: string;
    applicationOrder?: number;
    feeCharged?: string;
    minFee?: string;
}

export interface GetEventsResponse {
    events: Array<{
        type: string;
        ledger: number;
        ledgerClosedAt: string;
        id: string;
        contractId: string;
        topic: string[];
        value: {
            type: string;
            value: string;
        };
    }>;
    cursor?: string;
}

export interface LedgerEntryResponse {
    key: string;
    xdr: string;
    lastModifiedLedgerSeq?: number;
    liveUntilLedgerSeq?: number;
    isExtension?: boolean;
}

export interface GetLedgerEntriesResponse {
    entries: LedgerEntryResponse[];
    latestLedger: number;
    cursor?: string;
}

export interface FeeStatsResponse {
    lastLedger: number;
    lastLedgerBaseFeeInStroops: number;
    ledgerCapacityUsage: number;
    maxFee100Instructions: string;
    maxFee1KBStorage: string;
    maxFeeMode: string;
    maxFeeMin: string;
    minFeeA: string;
    minFeeB: string;
    congestion: {
        sosCount: string;
        ledgerCapacityUsageFraction: string;
        stellarCoreVersion: string;
    };
}

export interface VersionInfoResponse {
    version: string;
    gitHash: string;
    buildTimestamp: string;
    dependencies?: Record<string, string>;
}

export interface RPCClientConfig {
    urls: string[];
    timeout?: number;
    retries?: number;
    retryDelay?: number;
    circuitBreakerThreshold?: number;
    circuitBreakerTimeout?: number;
    maxRedirects?: number;
    headers?: Record<string, string>;
    validateRequests?: boolean;
    validateResponses?: boolean;
    enableMetrics?: boolean;
}

export interface RPCMetrics {
    totalRequests: number;
    totalSuccess: number;
    totalFailure: number;
    averageLatency: number;
    lastRequestAt?: number;
    endpoints: Record<string, EndpointMetrics>;
}

export interface EndpointMetrics {
    url: string;
    healthy: boolean;
    failureCount: number;
    circuitOpen: boolean;
    totalRequests: number;
    totalSuccess: number;
    totalFailure: number;
    averageDuration: number;
    lastSuccess?: number;
    lastFailure?: number;
}

export interface RequestValidationError {
    field: string;
    message: string;
}

export interface ResponseValidationError {
    method: string;
    message: string;
    expected?: string;
    actual?: string;
}
