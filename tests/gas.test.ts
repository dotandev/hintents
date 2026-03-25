// Copyright (c) Hintents Authors.
// SPDX-License-Identifier: Apache-2.0

import {
    extractGasEstimation,
    budgetToGasEstimation,
    isCpuWarning,
    isCpuCritical,
    isMemoryWarning,
    isMemoryCritical,
    hasBudgetPressure,
    formatGasEstimation,
    BASE_FEE_STROOPS,
    CPU_STROOPS_PER_UNIT,
    MEM_STROOPS_PER_UNIT,
    UPPER_BOUND_MULTIPLIER_PERCENT,
    BudgetUsage,
    SimulationResponse,
    GasEstimation,
} from '../src/gas';

// ─── Test helpers ─────────────────────────────────────────────────────────────

function makeBudget(overrides: Partial<BudgetUsage> = {}): BudgetUsage {
    return {
        cpu_instructions: 50_000_000,
        memory_bytes: 25_000_000,
        operations_count: 10,
        cpu_limit: 100_000_000,
        memory_limit: 50_000_000,
        cpu_usage_percent: 50.0,
        memory_usage_percent: 50.0,
        ...overrides,
    };
}

function makeResponse(overrides: Partial<SimulationResponse> = {}): SimulationResponse {
    return {
        status: 'success',
        budget_usage: makeBudget(),
        ...overrides,
    };
}

// ─── extractGasEstimation ─────────────────────────────────────────────────────

describe('extractGasEstimation', () => {
    it('extracts CPU and memory costs from a valid response', () => {
        const gas = extractGasEstimation(makeResponse());

        expect(gas.cpuCost).toBe(50_000_000);
        expect(gas.memoryCost).toBe(25_000_000);
        expect(gas.cpuLimit).toBe(100_000_000);
        expect(gas.memoryLimit).toBe(50_000_000);
        expect(gas.cpuUsagePercent).toBe(50.0);
        expect(gas.memoryUsagePercent).toBe(50.0);
        expect(gas.operationsCount).toBe(10);
    });

    it('computes fee estimates', () => {
        const gas = extractGasEstimation(makeResponse());

        expect(gas.estimatedFeeLowerBound).toBeGreaterThan(0);
        expect(gas.estimatedFeeUpperBound).toBeGreaterThanOrEqual(gas.estimatedFeeLowerBound);
    });

    it('throws on null response', () => {
        expect(() => extractGasEstimation(null)).toThrow('null or undefined');
    });

    it('throws on undefined response', () => {
        expect(() => extractGasEstimation(undefined)).toThrow('null or undefined');
    });

    it('throws when budget_usage is missing', () => {
        expect(() => extractGasEstimation({ status: 'success' })).toThrow('budget_usage');
    });
});

// ─── budgetToGasEstimation ────────────────────────────────────────────────────

describe('budgetToGasEstimation', () => {
    it('converts BudgetUsage to GasEstimation', () => {
        const bu = makeBudget();
        const gas = budgetToGasEstimation(bu);

        expect(gas.cpuCost).toBe(bu.cpu_instructions);
        expect(gas.memoryCost).toBe(bu.memory_bytes);
        expect(gas.operationsCount).toBe(bu.operations_count);
    });

    it('computes correct lower bound for zero usage', () => {
        const gas = budgetToGasEstimation(
            makeBudget({ cpu_instructions: 0, memory_bytes: 0 }),
        );
        expect(gas.estimatedFeeLowerBound).toBe(BASE_FEE_STROOPS);
    });

    it('computes correct lower bound for CPU-only usage', () => {
        const gas = budgetToGasEstimation(
            makeBudget({ cpu_instructions: 100_000, memory_bytes: 0 }),
        );
        const expectedCpu = Math.floor(100_000 / CPU_STROOPS_PER_UNIT);
        expect(gas.estimatedFeeLowerBound).toBe(BASE_FEE_STROOPS + expectedCpu);
    });

    it('computes correct lower bound for memory-only usage', () => {
        const gas = budgetToGasEstimation(
            makeBudget({ cpu_instructions: 0, memory_bytes: 128 * 1024 }),
        );
        const expectedMem = Math.floor((128 * 1024) / MEM_STROOPS_PER_UNIT);
        expect(gas.estimatedFeeLowerBound).toBe(BASE_FEE_STROOPS + expectedMem);
    });

    it('upper bound = floor(lower * 115 / 100)', () => {
        const gas = budgetToGasEstimation(makeBudget());
        const expected = Math.floor(
            (gas.estimatedFeeLowerBound * UPPER_BOUND_MULTIPLIER_PERCENT) / 100,
        );
        expect(gas.estimatedFeeUpperBound).toBe(expected);
    });
});

// ─── Threshold predicates ─────────────────────────────────────────────────────

describe('threshold helpers', () => {
    const cases: Array<{
        name: string;
        cpuPct: number;
        memPct: number;
        cpuWarn: boolean;
        cpuCrit: boolean;
        memWarn: boolean;
        memCrit: boolean;
        pressure: boolean;
    }> = [
        { name: 'low usage', cpuPct: 30, memPct: 20, cpuWarn: false, cpuCrit: false, memWarn: false, memCrit: false, pressure: false },
        { name: 'CPU warning', cpuPct: 82, memPct: 20, cpuWarn: true, cpuCrit: false, memWarn: false, memCrit: false, pressure: true },
        { name: 'mem warning', cpuPct: 20, memPct: 85, cpuWarn: false, cpuCrit: false, memWarn: true, memCrit: false, pressure: true },
        { name: 'CPU critical', cpuPct: 96, memPct: 20, cpuWarn: true, cpuCrit: true, memWarn: false, memCrit: false, pressure: true },
        { name: 'mem critical', cpuPct: 20, memPct: 97, cpuWarn: false, cpuCrit: false, memWarn: true, memCrit: true, pressure: true },
        { name: 'both critical', cpuPct: 99, memPct: 99, cpuWarn: true, cpuCrit: true, memWarn: true, memCrit: true, pressure: true },
        { name: 'exact warning boundary', cpuPct: 80, memPct: 80, cpuWarn: true, cpuCrit: false, memWarn: true, memCrit: false, pressure: true },
        { name: 'exact critical boundary', cpuPct: 95, memPct: 95, cpuWarn: true, cpuCrit: true, memWarn: true, memCrit: true, pressure: true },
    ];

    it.each(cases)('$name', ({ cpuPct, memPct, cpuWarn, cpuCrit, memWarn, memCrit, pressure }) => {
        const gas: GasEstimation = {
            cpuCost: 0,
            memoryCost: 0,
            cpuLimit: 100_000_000,
            memoryLimit: 50_000_000,
            cpuUsagePercent: cpuPct,
            memoryUsagePercent: memPct,
            operationsCount: 0,
            estimatedFeeLowerBound: 100,
            estimatedFeeUpperBound: 115,
        };

        expect(isCpuWarning(gas)).toBe(cpuWarn);
        expect(isCpuCritical(gas)).toBe(cpuCrit);
        expect(isMemoryWarning(gas)).toBe(memWarn);
        expect(isMemoryCritical(gas)).toBe(memCrit);
        expect(hasBudgetPressure(gas)).toBe(pressure);
    });
});

// ─── formatGasEstimation ──────────────────────────────────────────────────────

describe('formatGasEstimation', () => {
    it('produces a human-readable string', () => {
        const gas = budgetToGasEstimation(makeBudget());
        const s = formatGasEstimation(gas);

        expect(s).toContain('GasEstimation{');
        expect(s).toContain('CPU:');
        expect(s).toContain('Memory:');
        expect(s).toContain('Fee:');
        expect(s).toContain('stroops}');
    });

    it('includes [WARNING] when CPU is elevated', () => {
        const gas = budgetToGasEstimation(
            makeBudget({ cpu_usage_percent: 85, memory_usage_percent: 40 }),
        );
        const s = formatGasEstimation(gas);
        expect(s).toContain('[WARNING]');
    });

    it('includes [CRITICAL] when resources are critical', () => {
        const gas = budgetToGasEstimation(
            makeBudget({ cpu_usage_percent: 96, memory_usage_percent: 97 }),
        );
        const s = formatGasEstimation(gas);
        const count = (s.match(/\[CRITICAL\]/g) || []).length;
        expect(count).toBe(2);
    });
});

// ─── JSON round-trip ──────────────────────────────────────────────────────────

describe('JSON serialization', () => {
    it('round-trips through JSON.stringify / JSON.parse', () => {
        const original = budgetToGasEstimation(makeBudget());
        const json = JSON.stringify(original);
        const parsed: GasEstimation = JSON.parse(json);

        expect(parsed.cpuCost).toBe(original.cpuCost);
        expect(parsed.memoryCost).toBe(original.memoryCost);
        expect(parsed.estimatedFeeLowerBound).toBe(original.estimatedFeeLowerBound);
        expect(parsed.estimatedFeeUpperBound).toBe(original.estimatedFeeUpperBound);
    });
});
