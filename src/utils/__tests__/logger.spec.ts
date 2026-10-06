// Copyright (c) 2026 dotandev
// SPDX-License-Identifier: MIT OR Apache-2.0

import { Logger, LogLevel, LogCategory } from '../logger';
import * as fs from 'fs';
import * as path from 'path';

describe('Logger', () => {
    let logger: Logger;
    let consoleLogSpy: jest.SpyInstance;
    let consoleErrorSpy: jest.SpyInstance;
    let appendFileSpy: jest.SpyInstance | undefined;

    beforeEach(() => {
        logger = new Logger(LogLevel.STANDARD, false); // disable default file logging for standard tests
        consoleLogSpy = jest.spyOn(console, 'log').mockImplementation();
        consoleErrorSpy = jest.spyOn(console, 'error').mockImplementation();
        try {
            appendFileSpy = jest.spyOn(fs, 'appendFile').mockImplementation((_path, _data, callback) => {
                if (typeof callback === 'function') {
                    callback(null);
                }
            });
        } catch {
            // Fallback if appendFile spy cannot be attached directly in environment
            appendFileSpy = undefined;
        }
    });

    afterEach(() => {
        consoleLogSpy.mockRestore();
        consoleErrorSpy.mockRestore();
        if (appendFileSpy) {
            appendFileSpy.mockRestore();
            appendFileSpy = undefined;
        }
    });

    describe('log levels', () => {
        it('should not log verbose in standard mode', () => {
            logger.setLevel(LogLevel.STANDARD);
            logger.verbose(LogCategory.INFO, 'test message');

            expect(consoleLogSpy).not.toHaveBeenCalled();
        });

        it('should log verbose in verbose mode', () => {
            logger.setLevel(LogLevel.VERBOSE);
            logger.verbose(LogCategory.INFO, 'test message');

            expect(consoleLogSpy).toHaveBeenCalled();
        });

        it('should always log standard messages', () => {
            logger.setLevel(LogLevel.STANDARD);
            logger.info('test message');

            expect(consoleLogSpy).toHaveBeenCalledWith('test message');
        });

        it('should not log in silent mode', () => {
            logger.setLevel(LogLevel.SILENT);
            logger.info('test message');

            expect(consoleLogSpy).not.toHaveBeenCalled();
        });
    });

    describe('formatting', () => {
        it('should format bytes correctly', () => {
            expect(logger.formatBytes(0)).toBe('0 bytes');
            expect(logger.formatBytes(1024)).toBe('1.0 KB');
            expect(logger.formatBytes(1048576)).toBe('1.0 MB');
        });

        it('should format duration correctly', () => {
            expect(logger.formatDuration(500)).toBe('500ms');
            expect(logger.formatDuration(1500)).toBe('1.50s');
        });
    });

    describe('categories', () => {
        it('should use different colors for categories', () => {
            logger.setLevel(LogLevel.VERBOSE);

            logger.verbose(LogCategory.RPC, 'rpc message');
            logger.verbose(LogCategory.DATA, 'data message');
            logger.verbose(LogCategory.SIM, 'sim message');

            expect(consoleLogSpy).toHaveBeenCalledTimes(3);
        });
    });

    describe('error logging', () => {
        it('should log error message in standard mode', () => {
            logger.setLevel(LogLevel.STANDARD);
            logger.error('Error occurred');
            expect(consoleErrorSpy).toHaveBeenCalled();
        });

        it('should log stack trace in verbose mode', () => {
            logger.setLevel(LogLevel.VERBOSE);
            const error = new Error('Debug error');
            error.stack = 'stack trace content';
            logger.error('Error occurred', error);
            expect(consoleErrorSpy).toHaveBeenCalledTimes(3); // Error msg + "Stack trace:" + stack content
        });
    });

    describe('file logging', () => {
        const testLogDir = path.join(process.cwd(), '.erst_test_logs');
        const testLogFile = path.join(testLogDir, 'test.log');

        beforeEach(() => {
            try {
                if (fs.existsSync(testLogFile)) fs.unlinkSync(testLogFile);
                if (fs.existsSync(testLogDir)) fs.rmdirSync(testLogDir);
            } catch { }
        });

        afterEach(() => {
            try {
                if (fs.existsSync(testLogFile)) fs.unlinkSync(testLogFile);
                if (fs.existsSync(testLogDir)) fs.rmdirSync(testLogDir);
            } catch { }
        });

        it('should write logs to file when file logging is enabled', (done) => {
            logger.setLogFilePath(testLogFile);
            logger.setFileLoggingEnabled(true);
            logger.info('File log message');

            setTimeout(() => {
                expect(fs.existsSync(testLogFile)).toBe(true);
                const content = fs.readFileSync(testLogFile, 'utf8');
                expect(content).toContain('[INFO] File log message');
                done();
            }, 100);
        });

        it('should not write logs to file when file logging is disabled', (done) => {
            logger.setLogFilePath(testLogFile);
            logger.setFileLoggingEnabled(false);
            logger.info('File log message');

            setTimeout(() => {
                expect(fs.existsSync(testLogFile)).toBe(false);
                done();
            }, 100);
        });

        it('should handle filesystem errors gracefully without crashing', () => {
            logger.setFileLoggingEnabled(true);
            logger.setLogFilePath('\0invalid\path/extension.log');

            expect(() => {
                logger.info('Read-only test message');
            }).not.toThrow();
        });
    });
});

