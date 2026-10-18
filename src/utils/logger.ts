// Copyright (c) 2026 dotandev
// SPDX-License-Identifier: MIT OR Apache-2.0

import chalk from 'chalk';
import * as fs from 'fs';
import * as path from 'path';

export const LogLevel = {
    SILENT: 0,
    STANDARD: 1,
    VERBOSE: 2,
} as const;
export type LogLevel = (typeof LogLevel)[keyof typeof LogLevel];

export const LogCategory = {
    RPC: 'RPC',
    DATA: 'DATA',
    SIM: 'SIM',
    PERF: 'PERF',
    ERROR: 'ERROR',
    INFO: 'INFO',
} as const;
export type LogCategory = (typeof LogCategory)[keyof typeof LogCategory];

function getConfigFileLoggingEnabled(): boolean {
    try {
        const pkgPath = path.join(process.cwd(), 'package.json');
        if (fs.existsSync(pkgPath)) {
            const pkgContent = fs.readFileSync(pkgPath, 'utf8');
            const pkg = JSON.parse(pkgContent);
            if (pkg && pkg.config && typeof pkg.config.enableFileLogging === 'boolean') {
                return pkg.config.enableFileLogging;
            }
        }
    } catch {
        // Fall back to enabled by default if config reading fails
    }
    return true;
}

export class Logger {
    private level: LogLevel;
    private startTime: number;
    private fileLoggingEnabled: boolean;
    private logDirPath: string;
    private logFilePath: string;

    constructor(level: LogLevel = LogLevel.STANDARD, fileLoggingEnabled?: boolean) {
        this.level = level;
        this.startTime = Date.now();
        this.fileLoggingEnabled = fileLoggingEnabled ?? getConfigFileLoggingEnabled();
        this.logDirPath = path.join(process.cwd(), '.erst');
        this.logFilePath = path.join(this.logDirPath, 'extension.log');
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
     * Enable or disable persistent file logging
     */
    setFileLoggingEnabled(enabled: boolean): void {
        this.fileLoggingEnabled = enabled;
    }

    /**
     * Check if persistent file logging is enabled
     */
    isFileLoggingEnabled(): boolean {
        return this.fileLoggingEnabled;
    }

    /**
     * Set custom log file path (useful for testing or overrides)
     */
    setLogFilePath(customPath: string): void {
        this.logFilePath = customPath;
        this.logDirPath = path.dirname(customPath);
    }

    /**
     * Append formatted message to persistent log file (.erst/extension.log)
     */
    private writeToFile(message: string): void {
        if (!this.fileLoggingEnabled) {
            return;
        }

        try {
            if (!fs.existsSync(this.logDirPath)) {
                fs.mkdirSync(this.logDirPath, { recursive: true });
            }

            const cleanMessage = message.replace(/\u001b\[\d+m/g, '');
            const timestamp = this.getTimestamp();
            const logEntry = `${timestamp} ${cleanMessage}\n`;

            fs.appendFile(this.logFilePath, logEntry, (err: Error | null) => {
                // Gracefully ignore filesystem write errors (e.g. read-only filesystem)
            });
        } catch {
            // Gracefully ignore filesystem errors (e.g. read-only directory creation failure)
        }
    }

    /**
     * Log standard message (always shown unless silent)
     */
    info(message: string): void {
        if (this.level >= LogLevel.STANDARD) {
            console.log(message);
            this.writeToFile(`[INFO] ${message}`);
        }
    }

    /**
     * Log success message
     */
    success(message: string): void {
        if (this.level >= LogLevel.STANDARD) {
            console.log(chalk.green(' ' + message));
            this.writeToFile(`[SUCCESS] ${message}`);
        }
    }

    /**
     * Log warning message
     */
    warn(message: string): void {
        if (this.level >= LogLevel.STANDARD) {
            console.log(chalk.yellow('[WARN]  ' + message));
            this.writeToFile(`[WARN] ${message}`);
        }
    }

    /**
     * Log error message
     */
    error(message: string, error?: Error): void {
        if (this.level >= LogLevel.STANDARD) {
            const errDetails = error ? `: ${error.message}` : '';
            console.error(chalk.red('[FAIL] ' + message + errDetails));
            this.writeToFile(`[FAIL] ${message}${errDetails}`);

            if (error && this.isVerbose()) {
                console.error(chalk.red('   Stack trace:'));
                console.error(chalk.gray(error.stack || error.message));
                this.writeToFile(`   Stack trace: ${error.stack || error.message}`);
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
            this.writeToFile(`[${category}] ${message}`);
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
            this.writeToFile(`[${category}]${spaces}${message}`);
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

