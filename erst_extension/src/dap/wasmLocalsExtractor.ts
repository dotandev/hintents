// Copyright (c) Hintents Authors.
// SPDX-License-Identifier: Apache-2.0

/**
 * WASM Locals Extractor
 *
 * Extracts and manages local variables from WASM debug information (DWARF).
 * This module parses source maps and debug symbols to provide readable
 * variable names and types at debug time.
 */

/**
 * Represents a local variable in WASM code
 */
export interface WasmLocal {
    name: string;
    type: string;
    location: string;
    value?: any;
    startLine?: number;
    endLine?: number;
}

/**
 * Represents a WASM locals scope at a specific instruction
 */
export interface WasmLocalsScope {
    instruction: number;
    contractId?: string;
    functionName?: string;
    locals: WasmLocal[];
    sourceFile?: string;
    sourceLine?: number;
}

/**
 * Manages extraction and caching of WASM locals from debug information
 */
export class WasmLocalsExtractor {
    private wasmBytes: Buffer | null = null;
    private localsCache = new Map<string, WasmLocal[]>();
    private hasDebugSymbols: boolean = false;

    constructor(wasmBytes?: Buffer) {
        if (wasmBytes) {
            this.setWasmData(wasmBytes);
        }
    }

    /**
     * Sets the WASM binary data for locals extraction
     */
    setWasmData(wasmBytes: Buffer): void {
        this.wasmBytes = wasmBytes;
        this.hasDebugSymbols = this.checkDebugSymbols(wasmBytes);
        this.localsCache.clear();
    }

    /**
     * Checks if the WASM binary contains debug symbols
     */
    private checkDebugSymbols(wasmBytes: Buffer): boolean {
        // Check for DWARF debug sections in the WASM binary
        // WASM debug sections are typically in custom sections named ".debug_*"
        const wasmMagic = Buffer.from([0x00, 0x61, 0x73, 0x6d]); // "\0asm"
        if (!wasmBytes.slice(0, 4).equals(wasmMagic)) {
            return false;
        }

        // Simple heuristic: search for common DWARF section names
        const wasmStr = wasmBytes.toString('binary');
        const hasDebugInfo =
            wasmStr.includes('.debug_info') ||
            wasmStr.includes('.debug_line') ||
            wasmStr.includes('.debug_abbrev');

        return hasDebugInfo;
    }

    /**
     * Extracts locals for a given instruction offset
     * Returns cached results if available
     */
    extractLocalsAtInstruction(instruction: number, contractId?: string, functionName?: string): WasmLocalsScope {
        const cacheKey = `${instruction}:${contractId || ''}:${functionName || ''}`;
        const cached = this.localsCache.get(cacheKey);

        if (cached) {
            return {
                instruction,
                contractId,
                functionName,
                locals: cached,
            };
        }

        const locals = this.parseLocalsForInstruction(instruction);
        this.localsCache.set(cacheKey, locals);

        return {
            instruction,
            contractId,
            functionName,
            locals,
        };
    }

    /**
     * Parses local variables for a specific instruction
     * This is a simplified implementation that can be extended with full DWARF parsing
     */
    private parseLocalsForInstruction(_instruction: number): WasmLocal[] {
        if (!this.hasDebugSymbols || !this.wasmBytes) {
            return [];
        }

        // TODO: Implement full DWARF parsing
        // For now, return empty array
        // Full implementation would:
        // 1. Parse .debug_info section
        // 2. Find DIE (Debugging Information Entry) for the function
        // 3. Extract local variable entries
        // 4. Resolve type information from .debug_info
        // 5. Match against instruction range

        return [];
    }

    /**
     * Extracts locals from the execution trace step data
     * Used when locals are provided directly from the simulator
     */
    extractLocalsFromStepData(stepData: any): WasmLocal[] {
        if (!stepData || typeof stepData !== 'object') {
            return [];
        }

        const locals: WasmLocal[] = [];

        // If the trace step already contains wasm_locals, extract them
        if (Array.isArray(stepData.wasm_locals)) {
            return stepData.wasm_locals.map((local: any) => ({
                name: local.name || 'unknown',
                type: local.type || 'any',
                location: local.location || '',
                value: local.value,
                startLine: local.startLine,
                endLine: local.endLine,
            }));
        }

        // Otherwise, try to infer locals from available data
        if (stepData.arguments && Array.isArray(stepData.arguments)) {
            stepData.arguments.forEach((arg: any, idx: number) => {
                locals.push({
                    name: `arg${idx}`,
                    type: 'any',
                    location: `argument ${idx}`,
                    value: arg,
                });
            });
        }

        return locals;
    }

    /**
     * Gets debug information availability status
     */
    hasDebugInfo(): boolean {
        return this.hasDebugSymbols;
    }

    /**
     * Clears the locals cache
     */
    clearCache(): void {
        this.localsCache.clear();
    }
}
