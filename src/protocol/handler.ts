import { URIParser, ParsedURI } from './uri-parser';
import { DebugSession } from '../debug/session';
import * as fs from 'fs/promises';
import * as path from 'path';
import * as os from 'os';

interface HandlerConfig {
    secret?: string;
    trustedOrigins?: string[];
    rateLimit?: {
        maxInvocations: number;
        windowMs: number;
    };
}

/**
 * ProtocolHandler orchestrates the processing of incoming erst:// URIs.
 * It handles sanitization, parsing, security validations, and session initialization.
 */
export class ProtocolHandler {
    private parser: URIParser;
    private config: HandlerConfig;
    private invocationLog: Map<string, number[]> = new Map();
    private auditLogPath: string;

    constructor(config: HandlerConfig = {}) {
        this.parser = new URIParser();
        this.config = {
            rateLimit: {
                maxInvocations: 5,
                windowMs: 60000, // 1 minute window
            },
            ...config,
        };

        // Setup audit log path in the user's home directory
        this.auditLogPath = path.join(os.homedir(), '.erst', 'protocol-audit.log');
    }

    /**
     * Handle an incoming protocol URI string
     */
    async handle(uriString: string): Promise<void> {
        const timestamp = new Date().toISOString();

        try {
            // 1. Sanitize incoming input early
            const sanitized = this.parser.sanitize(uriString);

            // 2. Parse the URI into components
            const parsed = this.parser.parse(sanitized);

            // 3. Execute security validations
            await this.securityChecks(parsed);

            // 4. Log the successful invocation
            await this.logAudit('ACCEPTED', parsed, timestamp);

            // 5. Initialize the debug session
            await this.startDebugSession(parsed);

        } catch (error) {
            if (error instanceof Error) {
                await this.logAudit('REJECTED', { raw: uriString, error: error.message }, timestamp);
                throw error;
            }
            throw new Error('An unknown error occurred during protocol handling');
        }
    }

    /**
     * Perform a suite of security validations on the parsed URI
     */
    private async securityChecks(parsed: ParsedURI): Promise<void> {
        // 1. Rate limiting based on the raw URI (prevent spamming)
        this.checkRateLimit(parsed.raw);

        // 2. Signature verification if a secret key is provided
        if (this.config.secret && parsed.signature) {
            const isValid = this.parser.validateSignature(parsed, this.config.secret);

            if (!isValid) {
                throw new Error('Security verification failed: Invalid signature');
            }
        }

        // 3. Origin validation if sources are restricted
        if (this.config.trustedOrigins) {
            if (!parsed.source) {
                throw new Error('Access denied: Authentication source is required');
            }
            if (!this.config.trustedOrigins.includes(parsed.source)) {
                throw new Error(`Access denied: Untrusted origin '${parsed.source}'`);
            }
        }

        // Additional checks could be added here (e.g., blacklisted transaction hashes)
    }

    /**
     * Simple in-memory rate limiting mechanism
     */
    private checkRateLimit(uri: string): void {
        const now = Date.now();
        const key = uri;

        if (!this.invocationLog.has(key)) {
            this.invocationLog.set(key, []);
        }

        const timestamps = this.invocationLog.get(key)!;

        // Filter out timestamps that are outside the current time window
        const windowStart = now - (this.config.rateLimit?.windowMs || 60000);
        const recentTimestamps = timestamps.filter(ts => ts > windowStart);

        if (recentTimestamps.length >= (this.config.rateLimit?.maxInvocations || 5)) {
            throw new Error('Rate limit exceeded: Please wait before initiating another debug session');
        }

        // Log the current invocation
        recentTimestamps.push(now);
        this.invocationLog.set(key, recentTimestamps);
    }

    /**
     * Initialize and start a new debug session
     */
    private async startDebugSession(parsed: ParsedURI): Promise<void> {
        console.log('🔍 Initiating debug session from dashboard link...');
        console.log(`  └─ Transaction: ${parsed.transactionHash}`);
        console.log(`  └─ Network:     ${parsed.network}`);

        if (parsed.operation !== undefined) {
            console.log(`  └─ Operation Index: ${parsed.operation}`);
        }

        if (parsed.source) {
            console.log(`  └─ Source:      ${parsed.source}`);
        }

        const session = new DebugSession({
            transactionHash: parsed.transactionHash,
            network: parsed.network,
            operation: parsed.operation,
        });

        await session.start();
    }

    /**
     * Log protocol invocation attempts to a local audit file.
     * Records timestamp, status (ACCEPTED/REJECTED), and request context.
     */
    private async logAudit(status: 'ACCEPTED' | 'REJECTED', data: any, timestamp: string): Promise<void> {
        try {
            // Ensure the audit directory exists
            await fs.mkdir(path.dirname(this.auditLogPath), { recursive: true });

            const logEntry = JSON.stringify({
                timestamp,
                status,
                data,
            }) + '\n';

            // Append the log entry
            await fs.appendFile(this.auditLogPath, logEntry, 'utf8');
        } catch (error) {
            // Don't fail the operation if logging fails, but alert the dev
            console.error('CRITICAL: Failed to write to audit log:', error);
        }
    }
}
