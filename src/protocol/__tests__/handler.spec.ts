import { ProtocolHandler } from '../handler';

describe('ProtocolHandler', () => {
    let handler: ProtocolHandler;

    beforeEach(() => {
        handler = new ProtocolHandler({
            secret: 'test-secret',
            trustedOrigins: ['dashboard', 'explorer'],
            rateLimit: {
                maxInvocations: 3,
                windowMs: 1000,
            },
        });
    });

    describe('rate limiting', () => {
        it('should allow requests within the defined rate limit', async () => {
            const uri1 = 'erst://debug/a1b2c3d4e5f67890123456789abcdef0123456789abcdef0123456789abcdeff?network=testnet';
            const uri2 = 'erst://debug/b2c3d4e5f67890123456789abcdef0123456789abcdef0123456789abcdeffaa?network=testnet';

            // Should not throw any errors
            await expect(handler.handle(uri1)).resolves.not.toThrow();
            await expect(handler.handle(uri2)).resolves.not.toThrow();
        });

        it('should reject requests that exceed the rate limit', async () => {
            const uri = 'erst://debug/a1b2c3d4e5f67890123456789abcdef0123456789abcdef0123456789abcdeff?network=testnet';

            // First 3 requests should succeed
            await handler.handle(uri);
            await handler.handle(uri);
            await handler.handle(uri);

            // The 4th request should be rejected due to rate limiting
            await expect(handler.handle(uri)).rejects.toThrow('Rate limit exceeded');
        });
    });

    describe('origin validation', () => {
        it('should accept requests from trusted origins', async () => {
            const uri = 'erst://debug/a1b2c3d4e5f67890123456789abcdef0123456789abcdef0123456789abcdeff?network=testnet&source=dashboard';

            await expect(handler.handle(uri)).resolves.not.toThrow();
        });

        it('should reject requests from untrusted origins', async () => {
            const uri = 'erst://debug/a1b2c3d4e5f67890123456789abcdef0123456789abcdef0123456789abcdeff?network=testnet&source=malicious-site';

            await expect(handler.handle(uri)).rejects.toThrow('Access denied: Untrusted origin');
        });
    });
});
