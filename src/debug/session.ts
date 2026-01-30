/**
 * Mock DebugSession class to satisfy ProtocolHandler dependencies.
 * This will be replaced or integrated with real debug functionality later.
 */
export class DebugSession {
    private config: any;

    constructor(config: { transactionHash: string; network: string; operation?: number }) {
        this.config = config;
    }

    async start(): Promise<void> {
        console.log(`🚀 Debug session started for transaction: ${this.config.transactionHash}`);
        // Real implementation would connect to the network and fetch transaction data
    }
}
