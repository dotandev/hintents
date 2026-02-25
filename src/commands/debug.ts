// Copyright (c) 2026 dotandev
// SPDX-License-Identifier: MIT OR Apache-2.0

import { Command } from 'commander';
import { RPCConfigParser } from '../config/rpc-config';
import { FallbackRPCClient } from '../rpc/fallback-client';
import { getLogger, setLogLevel, LogLevel, LogCategory } from '../utils/logger';

export function registerDebugCommand(program: Command): void {
    program
        .command('debug <transaction>')
        .description('Debug a Stellar transaction with RPC fallback support')
        .option(
            '--rpc <urls>',
            'Comma-separated list of RPC URLs (e.g., https://rpc1.com,https://rpc2.com)',
        )
        .option('--timeout <ms>', 'Request timeout in milliseconds', '30000')
        .option('--retries <n>', 'Number of retries per endpoint', '3')
        .option('--verbose', 'Enable verbose output with detailed execution steps')
        .action(async (transaction: string, options) => {
            const startTime = Date.now();

            // Set log level based on verbose flag
            if (options.verbose) {
                setLogLevel(LogLevel.VERBOSE);
            } else {
                setLogLevel(LogLevel.STANDARD);
            }

            const logger = getLogger();

            try {
                // Load RPC configuration
                const config = RPCConfigParser.loadConfig({
                    rpc: options.rpc,
                    timeout: parseInt(options.timeout),
                    retries: parseInt(options.retries),
                });

                // Initialize RPC client with fallback
                const rpcClient = new FallbackRPCClient(config);

                // Standard output
                logger.info(`\n[SEARCH] Debugging transaction: ${transaction}\n`);

                // Verbose: Show configuration
                logger.verbose(LogCategory.INFO, 'Configuration');
                logger.verboseIndent(LogCategory.INFO, `RPC URL: ${options.rpc || 'Default'}`);
                logger.verboseIndent(LogCategory.INFO, `Transaction hash: ${transaction}`);
                logger.verboseIndent(LogCategory.INFO, `Verbose mode: enabled\n`);

                // Make RPC request
                logger.verbose(LogCategory.RPC, 'Initiating transaction fetch...');
                const txData = await rpcClient.request('/transactions/' + transaction, { method: 'GET' });

                // Verbose: Data parsing
                logger.verbose(LogCategory.DATA, 'Parsing transaction response...');
                logger.verboseIndent(LogCategory.DATA, `Ledger: ${txData.ledger || 'N/A'}`);
                logger.verboseIndent(LogCategory.DATA, `Source: ${txData.source_account || 'N/A'}`);

                logger.success('Transaction fetched successfully');
                logger.info(`Transaction data: ${JSON.stringify(txData, null, 2)}`);

                // Success
                logger.success('Debug complete');

                // Performance metrics (verbose)
                const totalDuration = Date.now() - startTime;
                const memUsage = process.memoryUsage();

                logger.verbose(LogCategory.PERF, 'Performance metrics');
                logger.verboseIndent(LogCategory.PERF, `Total execution time: ${totalDuration}ms`);
                logger.verboseIndent(LogCategory.PERF, `Memory usage: ${logger.formatBytes(memUsage.heapUsed)}`);
                logger.verboseIndent(LogCategory.PERF, `Peak memory: ${logger.formatBytes(memUsage.heapTotal)}`);

            } catch (error) {
                const totalDuration = Date.now() - startTime;
                if (error instanceof Error) {
                    logger.error('Debug failed', error);
                } else {
                    logger.error('Debug failed: An unknown error occurred');
                }

                logger.verbose(LogCategory.PERF, `Failed after ${totalDuration}ms`);
                process.exit(1);
            }
        });

    // Add health check command
    program
        .command('rpc:health')
        .description('Check health of all configured RPC endpoints')
        .option('--rpc <urls>', 'Comma-separated list of RPC URLs')
        .action(async (options) => {
            try {
                const config = RPCConfigParser.loadConfig({ rpc: options.rpc });
                const rpcClient = new FallbackRPCClient(config);

                await rpcClient.performHealthChecks();

                const status = rpcClient.getHealthStatus();

                console.log('\n[STATS] RPC Endpoint Status:\n');
                status.forEach((ep, idx) => {
                    const statusIcon = ep.healthy ? '' : '[FAIL]';
                    const circuit = ep.circuitOpen ? ' [CIRCUIT OPEN]' : '';
                    const successRate = ep.metrics.totalRequests > 0
                        ? ((ep.metrics.totalSuccess / ep.metrics.totalRequests) * 100).toFixed(1)
                        : '0.0';

                    console.log(`  [${idx + 1}] ${statusIcon} ${ep.url}${circuit}`);
                    console.log(`      Success Rate: ${successRate}% (${ep.metrics.totalSuccess}/${ep.metrics.totalRequests})`);
                    console.log(`      Avg Duration: ${ep.metrics.averageDuration}ms`);
                    console.log(`      Failures: ${ep.failureCount}`);
                });

            } catch (error) {
                if (error instanceof Error) {
                    console.error('[FAIL] Health check failed:', error.message);
                } else {
                    console.error('[FAIL] Health check failed: An unknown error occurred');
                }
                process.exit(1);
            }
        });
}
