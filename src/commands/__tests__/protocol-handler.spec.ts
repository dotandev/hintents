// Copyright (c) 2026 dotandev
// SPDX-License-Identifier: MIT OR Apache-2.0

import { registerProtocolCommands } from '../protocol-handler';
import { Command } from 'commander';

describe('Protocol Handler Commands', () => {
    let program: Command;

    beforeEach(() => {
        program = new Command();
    });

    describe('protocol:status command', () => {
        it('should be registered with correct description', () => {
            registerProtocolCommands(program);
            const statusCmd = program.commands.find(cmd => cmd.name() === 'protocol:status');
            expect(statusCmd).toBeDefined();
            if (statusCmd) {
                expect(statusCmd.description()).toContain('Check current registration status');
            }
        });
    });

    describe('protocol:register command', () => {
        it('should be registered with correct description', () => {
            registerProtocolCommands(program);
            const registerCmd = program.commands.find(cmd => cmd.name() === 'protocol:register');
            expect(registerCmd).toBeDefined();
            if (registerCmd) {
                expect(registerCmd.description()).toContain('Register');
            }
        });
    });

    describe('protocol:unregister command', () => {
        it('should be registered with correct description', () => {
            registerProtocolCommands(program);
            const unregisterCmd = program.commands.find(cmd => cmd.name() === 'protocol:unregister');
            expect(unregisterCmd).toBeDefined();
            if (unregisterCmd) {
                expect(unregisterCmd.description()).toContain('Unregister');
            }
        });
    });

    describe('protocol-handler internal command', () => {
        it('should be registered with correct description', () => {
            registerProtocolCommands(program);
            const handlerCmd = program.commands.find(cmd => cmd.name() === 'protocol-handler');
            expect(handlerCmd).toBeDefined();
            if (handlerCmd) {
                expect(handlerCmd.description()).toContain('Internal');
            }
        });

        it('should have --force option', () => {
            registerProtocolCommands(program);
            const handlerCmd = program.commands.find(cmd => cmd.name() === 'protocol-handler');
            expect(handlerCmd).toBeDefined();
            if (handlerCmd) {
                const forceOption = handlerCmd.options.find(opt => opt.long === '--force');
                expect(forceOption).toBeDefined();
            }
        });
    });

    describe('All protocol commands registered', () => {
        it('should register all four commands', () => {
            registerProtocolCommands(program);
            const protocolCmds = program.commands.filter(cmd =>
                cmd.name()?.startsWith('protocol')
            );
            expect(protocolCmds.length).toBeGreaterThanOrEqual(4);
        });
    });
});


