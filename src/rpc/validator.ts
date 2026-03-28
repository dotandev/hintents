// Copyright (c) 2026 dotandev
// SPDX-License-Identifier: MIT OR Apache-2.0

import {
    RPCMethod,
    RPCRequest,
    RPCResponse,
    RPCMethodParams,
    RequestValidationError,
    ResponseValidationError,
    BatchRequestItem,
} from './types-v2';

export class RPCRequestValidator {
    private static readonly VALID_METHODS: RPCMethod[] = [
        'getHealth',
        'getLatestLedger',
        'getTransaction',
        'getTransactions',
        'getLedgerEntries',
        'simulateTransaction',
        'sendTransaction',
        'getEvents',
        'getNetwork',
        'getFeeStats',
        'getVersionInfo',
    ];

    static validate<T extends RPCMethod>(request: RPCRequest<T>): RequestValidationError[] {
        const errors: RequestValidationError[] = [];

        if (!request.jsonrpc || request.jsonrpc !== '2.0') {
            errors.push({ field: 'jsonrpc', message: 'Must be "2.0"' });
        }

        if (!request.method || !this.VALID_METHODS.includes(request.method)) {
            errors.push({ field: 'method', message: `Invalid method. Valid: ${this.VALID_METHODS.join(', ')}` });
        }

        if (request.id === undefined) {
            errors.push({ field: 'id', message: 'id is required' });
        }

        if (request.params) {
            const paramErrors = this.validateParams(request.method, request.params);
            errors.push(...paramErrors);
        }

        return errors;
    }

    private static validateParams<T extends RPCMethod>(
        method: T,
        params: RPCMethodParams[T]
    ): RequestValidationError[] {
        const errors: RequestValidationError[] = [];

        switch (method) {
            case 'getTransaction':
                if (params && typeof params === 'object' && 'hash' in params) {
                    if (!params.hash || typeof params.hash !== 'string') {
                        errors.push({ field: 'params.hash', message: 'hash must be a non-empty string' });
                    }
                }
                break;

            case 'getEvents':
                if (params && typeof params === 'object') {
                    if (!('startLedger' in params)) {
                        errors.push({ field: 'params.startLedger', message: 'startLedger is required' });
                    } else if (typeof params.startLedger !== 'number') {
                        errors.push({ field: 'params.startLedger', message: 'startLedger must be a number' });
                    }
                }
                break;

            case 'getLedgerEntries':
                if (params && typeof params === 'object' && 'keys' in params) {
                    if (!Array.isArray(params.keys)) {
                        errors.push({ field: 'params.keys', message: 'keys must be an array' });
                    }
                }
                break;

            case 'simulateTransaction':
            case 'sendTransaction':
                if (params && typeof params === 'object' && 'transaction' in params) {
                    if (!params.transaction || typeof params.transaction !== 'string') {
                        errors.push({ field: 'params.transaction', message: 'transaction must be a non-empty string' });
                    }
                }
        }

        return errors;
    }

    static validateBatchRequest(requests: BatchRequestItem[]): RequestValidationError[] {
        const errors: RequestValidationError[] = [];

        if (!Array.isArray(requests) || requests.length === 0) {
            errors.push({ field: 'requests', message: 'requests must be a non-empty array' });
            return errors;
        }

        if (requests.length > 100) {
            errors.push({ field: 'requests', message: 'Maximum 100 requests per batch' });
        }

        for (let i = 0; i < requests.length; i++) {
            const req = requests[i];
            if (!req.method || !this.VALID_METHODS.includes(req.method)) {
                errors.push({ field: `requests[${i}].method`, message: 'Invalid method' });
            }
            if (req.id === undefined) {
                errors.push({ field: `requests[${i}].id`, message: 'id is required' });
            }
        }

        return errors;
    }
}

export class RPCResponseValidator {
    static validate(response: unknown, expectedMethod?: string): ResponseValidationError[] {
        const errors: ResponseValidationError[] = [];

        if (!response || typeof response !== 'object') {
            errors.push({ method: expectedMethod || 'unknown', message: 'Response must be an object' });
            return errors;
        }

        const resp = response as Record<string, unknown>;

        if (!resp.jsonrpc || resp.jsonrpc !== '2.0') {
            errors.push({ method: expectedMethod || 'unknown', message: 'Invalid jsonrpc version' });
        }

        if (!('result' in resp || 'error' in resp)) {
            errors.push({ method: expectedMethod || 'unknown', message: 'Response must have result or error' });
        }

        if ('error' in resp && resp.error) {
            const error = resp.error as Record<string, unknown>;
            if (!error.code || !error.message) {
                errors.push({ method: expectedMethod || 'unknown', message: 'Error must have code and message' });
            }
        }

        if ('result' in resp && expectedMethod) {
            const result = resp.result;
            const resultErrors = this.validateResult(expectedMethod, result);
            errors.push(...resultErrors);
        }

        return errors;
    }

    private static validateResult(method: string, result: unknown): ResponseValidationError[] {
        const errors: ResponseValidationError[] = [];

        switch (method) {
            case 'getTransaction':
                if (result && typeof result === 'object') {
                    const tx = result as Record<string, unknown>;
                    if (!tx.hash) {
                        errors.push({ method, message: 'Transaction result missing hash' });
                    }
                }
                break;

            case 'getHealth':
                if (result && typeof result === 'object') {
                    const health = result as Record<string, unknown>;
                    if (!health.status) {
                        errors.push({ method, message: 'Health result missing status' });
                    }
                }
                break;

            case 'getEvents':
                if (result && typeof result === 'object') {
                    const events = result as Record<string, unknown>;
                    if (!Array.isArray(events.events)) {
                        errors.push({ method, message: 'Events result missing events array' });
                    }
                }
                break;
        }

        return errors;
    }

    static validateBatchResponse(responses: unknown[]): ResponseValidationError[] {
        const errors: ResponseValidationError[] = [];

        if (!Array.isArray(responses)) {
            errors.push({ method: 'batch', message: 'Batch response must be an array' });
            return errors;
        }

        for (let i = 0; i < responses.length; i++) {
            const respErrors = this.validate(responses[i]);
            respErrors.forEach(e => ({ ...e, method: `batch[${i}].${e.method}` }));
            errors.push(...respErrors);
        }

        return errors;
    }
}
