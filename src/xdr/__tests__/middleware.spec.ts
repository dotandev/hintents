// Copyright (c) 2026 dotandev
// SPDX-License-Identifier: MIT OR Apache-2.0

import { SDKContext, SDKResponse, SDKMiddleware, NextFn, composeMiddleware } from '../types';

describe('SDK Middleware Types', () => {
    const makeCtx = (overrides: Partial<SDKContext> = {}): SDKContext => ({
        path: '/test',
        method: 'POST',
        metadata: {},
        ...overrides,
    });

    const makeCore = <T>(value: T, status = 200): NextFn<T> => {
        return async (ctx) => ({
            data: value,
            status,
            duration: 1,
            endpoint: 'https://rpc.test',
            metadata: { ...ctx.metadata },
        });
    };

    it('should pass through with no middleware', async () => {
        const core = makeCore({ ok: true });
        const chain = composeMiddleware([], core);
        const res = await chain(makeCtx());
        expect(res.data).toEqual({ ok: true });
        expect(res.status).toBe(200);
    });

    it('should execute a single middleware', async () => {
        const mw: SDKMiddleware = async (ctx, next) => {
            ctx.metadata['touched'] = true;
            return next(ctx);
        };

        const core = makeCore('result');
        const chain = composeMiddleware([mw], core);
        const ctx = makeCtx();
        const res = await chain(ctx);

        expect(res.data).toBe('result');
        expect(ctx.metadata['touched']).toBe(true);
    });

    it('should execute middleware in order (first registered runs first)', async () => {
        const order: number[] = [];

        const mw1: SDKMiddleware = async (ctx, next) => {
            order.push(1);
            const res = await next(ctx);
            order.push(4);
            return res;
        };
        const mw2: SDKMiddleware = async (ctx, next) => {
            order.push(2);
            const res = await next(ctx);
            order.push(3);
            return res;
        };

        const core = makeCore(null);
        const chain = composeMiddleware([mw1, mw2], core);
        await chain(makeCtx());

        expect(order).toEqual([1, 2, 3, 4]);
    });

    it('should allow middleware to modify the response', async () => {
        const mw: SDKMiddleware = async (ctx, next) => {
            const res = await next(ctx);
            return { ...res, data: { ...res.data, injected: true } };
        };

        const core = makeCore({ original: true });
        const chain = composeMiddleware([mw], core);
        const res = await chain(makeCtx());

        expect(res.data).toEqual({ original: true, injected: true });
    });

    it('should allow middleware to short-circuit without calling next', async () => {
        const mw: SDKMiddleware = async (_ctx, _next) => ({
            data: 'cached',
            status: 200,
            duration: 0,
            endpoint: 'cache',
            metadata: {},
        });

        const coreCalled = jest.fn();
        const core: NextFn = async (ctx) => {
            coreCalled();
            return { data: null, status: 200, duration: 1, endpoint: '', metadata: {} };
        };

        const chain = composeMiddleware([mw], core);
        const res = await chain(makeCtx());

        expect(res.data).toBe('cached');
        expect(coreCalled).not.toHaveBeenCalled();
    });

    it('should propagate errors from middleware', async () => {
        const mw: SDKMiddleware = async () => {
            throw new Error('middleware error');
        };

        const core = makeCore(null);
        const chain = composeMiddleware([mw], core);

        await expect(chain(makeCtx())).rejects.toThrow('middleware error');
    });

    it('should allow middleware to modify request context', async () => {
        const authMiddleware: SDKMiddleware = async (ctx, next) => {
            ctx.headers = { ...ctx.headers, Authorization: 'Bearer token' };
            return next(ctx);
        };

        let capturedCtx: SDKContext | undefined;
        const core: NextFn = async (ctx) => {
            capturedCtx = ctx;
            return { data: null, status: 200, duration: 1, endpoint: '', metadata: {} };
        };

        const chain = composeMiddleware([authMiddleware], core);
        await chain(makeCtx());

        expect(capturedCtx?.headers?.Authorization).toBe('Bearer token');
    });
});

describe('composeMiddleware benchmark', () => {
    it('should compose and execute 100 middleware in under 50ms', async () => {
        const middlewares: SDKMiddleware[] = [];
        for (let i = 0; i < 100; i++) {
            middlewares.push(async (ctx, next) => next(ctx));
        }

        const core: NextFn = async (ctx) => ({
            data: null,
            status: 200,
            duration: 0,
            endpoint: '',
            metadata: {},
        });

        const chain = composeMiddleware(middlewares, core);
        const ctx: SDKContext = { path: '/', method: 'POST', metadata: {} };

        const start = performance.now();
        const iterations = 1000;
        for (let i = 0; i < iterations; i++) {
            await chain({ ...ctx, metadata: {} });
        }
        const elapsed = performance.now() - start;
        const avgMs = elapsed / iterations;

        // Each chain execution should complete well under 50ms
        expect(avgMs).toBeLessThan(50);
    });
});
