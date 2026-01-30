// Copyright (c) 2026 dotandev
// SPDX-License-Identifier: MIT OR Apache-2.0

import chalk from 'chalk';

export enum LogLevel {
    SILENT = 0,
    STANDARD = 1,
    VERBOSE = 2,
}

export enum LogCategory {
    RPC = 'RPC',
    DATA = 'DATA',
    SIM = 'SIM',
    PERF = 'PERF',
    ERROR = 'ERROR',
    INFO = 'INFO',
}

export class Logger {
    private level: LogLevel;
    private startTime: number;

    constructor(level: LogLevel = LogLevel.STANDARD) {
        this.level = level;
        this.startTime = Date.now();
    }

    /**
     * Set log level
     */
    setLevel(level: LogLevel): void {
        this.level = level;
    }

    /**
     * Check if verbose mode is enabled
     */
    isVerbose(): boolean {
        return this.level >= LogLevel.VERBOSE;
    }

    /**
     * Log standard message (always shown unless silent)
     */
    info(message: string): void {
        if (this.level >= LogLevel.STANDARD) {
            console.log(message);
        }
    }

    /**
     * Log success message
     */
    success(message: string): void {
        if (this.level >= LogLevel.STANDARD) {
            console.log(chalk.green('✅ ' + message));
        }
    }

    /**
     * Log warning message
     */
    warn(message: string): void {
        if (this.level >= LogLevel.STANDARD) {
            console.log(chalk.yellow('⚠️  ' + message));
        }
    }

    /**
     * Log error message
     */
    error(message: string, error?: Error): void {
        if (this.level >= LogLevel.STANDARD) {
            console.error(chalk.red('❌ ' + message + (error ? `: ${error.message}` : '')));

            if (error && this.isVerbose()) {
                console.error(chalk.red('   Stack trace:'));
                console.error(chalk.gray(error.stack || error.message));
            }
        }
    }

    /**
     * Log verbose message (only in verbose mode)
     */
    verbose(category: LogCategory, message: string): void {
        if (this.level >= LogLevel.VERBOSE) {
            const timestamp = this.getTimestamp();
            const categoryColor = this.getCategoryColor(category);
            const formattedCategory = chalk.bold(categoryColor(`[${category}]`));

            console.log(`${chalk.gray(timestamp)} ${formattedCategory} ${message}`);
        }
    }

    /**
     * Log verbose with indentation
     */
    verboseIndent(category: LogCategory, message: string, indent: number = 1): void {
        if (this.level >= LogLevel.VERBOSE) {
            const timestamp = this.getTimestamp();
            const categoryColor = this.getCategoryColor(category);
            const formattedCategory = chalk.bold(categoryColor(`[${category}]`));
            const spaces = '  '.repeat(indent);

            console.log(`${chalk.gray(timestamp)} ${formattedCategory}${spaces}${message}`);
        }
    }

    /**
     * Get elapsed time since logger start
     */
    private getTimestamp(): string {
        const elapsed = Date.now() - this.startTime;
        const minutes = Math.floor(elapsed / 60000);
        const seconds = Math.floor((elapsed % 60000) / 1000);
        const ms = elapsed % 1000;

        return `[${String(minutes).padStart(2, '0')}:${String(seconds).padStart(2, '0')}.${String(ms).padStart(3, '0')}]`;
    }

    // TODO: Add support for logging to a file in the future

    /**
     * Get color for category
     */
    private getCategoryColor(category: LogCategory): chalk.Chalk {
        switch (category) {
            case LogCategory.RPC:
                return chalk.blue;
            case LogCategory.DATA:
                return chalk.cyan;
            case LogCategory.SIM:
                return chalk.magenta;
            case LogCategory.PERF:
                return chalk.yellow;
            case LogCategory.ERROR:
                return chalk.red;
            default:
                return chalk.white;
        }
    }

    /**
     * Format bytes to human-readable size
     */
    formatBytes(bytes: number): string {
        if (bytes === 0) return '0 bytes';

        const k = 1024;
        const sizes = ['bytes', 'KB', 'MB', 'GB'];
        const i = Math.floor(Math.log(bytes) / Math.log(k));

        return `${(bytes / Math.pow(k, i)).toFixed(1)} ${sizes[i]}`;
    }

    /**
     * Format duration in milliseconds
     */
    formatDuration(ms: number): string {
        if (ms < 1000) {
            return `${ms}ms`;
        }
        return `${(ms / 1000).toFixed(2)}s`;
    }
}

// Global logger instance
let globalLogger: Logger | null = null;

export function getLogger(): Logger {
    if (!globalLogger) {
        globalLogger = new Logger();
    }
    return globalLogger;
}

export function setLogLevel(level: LogLevel): void {
    getLogger().setLevel(level);
}
