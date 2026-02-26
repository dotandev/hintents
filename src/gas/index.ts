// Copyright (c) Hintents Authors.
// SPDX-License-Identifier: Apache-2.0

/**
 * @module gas
 *
 * Provides programmatic access to simulated CPU and memory costs independently,
 * without parsing the full JSON simulation trace.
 *
 * Usage:
 * ```ts
 * import { extractGasEstimation, GasEstimation } from './gas';
 *
 * const gas = extractGasEstimation(simulationResponse);
 * console.log(gas.cpuCost);                // raw CPU instructions
 * console.log(gas.memoryCost);             // raw memory bytes
 * console.log(gas.estimatedFeeLowerBound); // conservative fee in stroops
 * ```
 */

// ─── Fee estimation constants ─────────────────────────────────────────────────

/** Minimum network base fee per transaction (stroops). */
export const BASE_FEE_STROOPS = 100;

/** CPU instructions per stroop (1 stroop per 10 000 insns). */
export const CPU_STROOPS_PER_UNIT = 10_000;

/** Memory bytes per stroop (1 stroop per 64 KiB). */
export const MEM_STROOPS_PER_UNIT = 64 * 1024;

/** Safety margin multiplier (115 %). */
export const UPPER_BOUND_MULTIPLIER_PERCENT = 115;

/** CPU usage percentage at which a warning is raised. */
export const CPU_WARNING_PERCENT = 80.0;

/** CPU usage percentage at which the situation is critical. */
export const CPU_CRITICAL_PERCENT = 95.0;

/** Memory usage percentage at which a warning is raised. */
export const MEM_WARNING_PERCENT = 80.0;

/** Memory usage percentage at which the situation is critical. */
export const MEM_CRITICAL_PERCENT = 95.0;

// ─── Types ────────────────────────────────────────────────────────────────────

/** Budget usage data as returned by the Rust simulator. */
export interface BudgetUsage {
    cpu_instructions: number;
    memory_bytes: number;
    operations_count: number;
    cpu_limit: number;
    memory_limit: number;
    cpu_usage_percent: number;
    memory_usage_percent: number;
}

/** Subset of SimulationResponse relevant to gas extraction. */
export interface SimulationResponse {
    status: string;
    error?: string;
    budget_usage?: BudgetUsage;
    // Other fields (events, logs, etc.) intentionally omitted — callers
    // pass the full response and we extract only what we need.
    [key: string]: unknown;
}

/**
 * GasEstimation provides a clean, focused view of simulated resource costs.
 */
export interface GasEstimation {
    /** Number of CPU instructions consumed. */
    cpuCost: number;
    /** Number of memory bytes consumed. */
    memoryCost: number;
    /** Maximum allowed CPU instructions. */
    cpuLimit: number;
    /** Maximum allowed memory bytes. */
    memoryLimit: number;
    /** Percentage of the CPU budget consumed (0–100). */
    cpuUsagePercent: number;
    /** Percentage of the memory budget consumed (0–100). */
    memoryUsagePercent: number;
    /** Number of host-function operations executed. */
    operationsCount: number;
    /** Conservative lower-bound fee estimate (stroops). */
    estimatedFeeLowerBound: number;
    /** Upper-bound fee estimate with safety margin (stroops). */
    estimatedFeeUpperBound: number;
}

// ─── Core functions ───────────────────────────────────────────────────────────

/**
 * Extract a {@link GasEstimation} from a simulator response.
 *
 * @throws {Error} if the response is nullish or lacks `budget_usage`.
 */
export function extractGasEstimation(resp: SimulationResponse | null | undefined): GasEstimation {
    if (!resp) {
        throw new Error('simulation response is null or undefined');
    }
    if (!resp.budget_usage) {
        throw new Error('simulation response does not contain budget_usage');
    }
    return budgetToGasEstimation(resp.budget_usage);
}

/**
 * Convert a raw {@link BudgetUsage} object to a {@link GasEstimation}.
 */
export function budgetToGasEstimation(bu: BudgetUsage): GasEstimation {
    const lower = estimateFee(bu.cpu_instructions, bu.memory_bytes);
    const upper = Math.floor((lower * UPPER_BOUND_MULTIPLIER_PERCENT) / 100);

    return {
        cpuCost: bu.cpu_instructions,
        memoryCost: bu.memory_bytes,
        cpuLimit: bu.cpu_limit,
        memoryLimit: bu.memory_limit,
        cpuUsagePercent: bu.cpu_usage_percent,
        memoryUsagePercent: bu.memory_usage_percent,
        operationsCount: bu.operations_count,
        estimatedFeeLowerBound: lower,
        estimatedFeeUpperBound: upper,
    };
}

// ─── Helper predicates ────────────────────────────────────────────────────────

/** Returns true when CPU usage >= warning threshold (80 %). */
export function isCpuWarning(gas: GasEstimation): boolean {
    return gas.cpuUsagePercent >= CPU_WARNING_PERCENT;
}

/** Returns true when CPU usage >= critical threshold (95 %). */
export function isCpuCritical(gas: GasEstimation): boolean {
    return gas.cpuUsagePercent >= CPU_CRITICAL_PERCENT;
}

/** Returns true when memory usage >= warning threshold (80 %). */
export function isMemoryWarning(gas: GasEstimation): boolean {
    return gas.memoryUsagePercent >= MEM_WARNING_PERCENT;
}

/** Returns true when memory usage >= critical threshold (95 %). */
export function isMemoryCritical(gas: GasEstimation): boolean {
    return gas.memoryUsagePercent >= MEM_CRITICAL_PERCENT;
}

/** Returns true when either CPU or memory is at warning level or above. */
export function hasBudgetPressure(gas: GasEstimation): boolean {
    return isCpuWarning(gas) || isMemoryWarning(gas);
}

/**
 * Format a gas estimation as a human-readable single-line string.
 */
export function formatGasEstimation(gas: GasEstimation): string {
    let cpuInd = '';
    if (isCpuCritical(gas)) {
        cpuInd = ' [CRITICAL]';
    } else if (isCpuWarning(gas)) {
        cpuInd = ' [WARNING]';
    }

    let memInd = '';
    if (isMemoryCritical(gas)) {
        memInd = ' [CRITICAL]';
    } else if (isMemoryWarning(gas)) {
        memInd = ' [WARNING]';
    }

    return (
        `GasEstimation{CPU: ${gas.cpuCost}/${gas.cpuLimit} (${gas.cpuUsagePercent.toFixed(1)}%)${cpuInd}, ` +
        `Memory: ${gas.memoryCost}/${gas.memoryLimit} (${gas.memoryUsagePercent.toFixed(1)}%)${memInd}, ` +
        `Ops: ${gas.operationsCount}, ` +
        `Fee: ${gas.estimatedFeeLowerBound}–${gas.estimatedFeeUpperBound} stroops}`
    );
}

// ─── Internal ─────────────────────────────────────────────────────────────────

function estimateFee(cpuInsns: number, memBytes: number): number {
    const cpu = Math.floor(cpuInsns / CPU_STROOPS_PER_UNIT);
    const mem = Math.floor(memBytes / MEM_STROOPS_PER_UNIT);
    return BASE_FEE_STROOPS + cpu + mem;
}
