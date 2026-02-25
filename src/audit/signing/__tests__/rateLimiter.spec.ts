import * as fs from 'fs';
import * as path from 'path';
import * as os from 'os';
import { HsmRateLimiter } from '../rateLimiter';

describe('HsmRateLimiter', () => {
    const limitFile = path.join(os.homedir(), '.erst', 'audit_hsm_calls.json');

    beforeEach(() => {
        if (fs.existsSync(limitFile)) {
            fs.unlinkSync(limitFile);
        }
        process.env.ERST_PKCS11_MAX_RPM = '5'; // Set a low limit for testing
    });

    afterAll(() => {
        if (fs.existsSync(limitFile)) {
            fs.unlinkSync(limitFile);
        }
        delete process.env.ERST_PKCS11_MAX_RPM;
    });

    it('should allow calls within the limit', async () => {
        for (let i = 0; i < 5; i++) {
            await expect(HsmRateLimiter.checkAndRecordCall()).resolves.not.toThrow();
        }
    });

    it('should throw an error when the limit is exceeded', async () => {
        for (let i = 0; i < 5; i++) {
            await HsmRateLimiter.checkAndRecordCall();
        }
        await expect(HsmRateLimiter.checkAndRecordCall()).rejects.toThrow(
            /HSM rate limit protection triggered/
        );
    });

    it('should allow calls again after the window has passed', async () => {
        // Mock Date.now to simulate time passing
        const originalNow = Date.now;
        let mockTime = 1000000;
        Date.now = jest.fn(() => mockTime);

        for (let i = 0; i < 5; i++) {
            await HsmRateLimiter.checkAndRecordCall();
        }

        await expect(HsmRateLimiter.checkAndRecordCall()).rejects.toThrow();

        // Advance time by 61 seconds
        mockTime += 61000;

        await expect(HsmRateLimiter.checkAndRecordCall()).resolves.not.toThrow();

        Date.now = originalNow;
    });
});
