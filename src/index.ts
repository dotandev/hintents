#!/usr/bin/env node

// Copyright (c) 2026 dotandev
// SPDX-License-Identifier: MIT OR Apache-2.0

import { Command } from 'commander';
import { registerProtocolCommands } from './commands/protocol-handler';
import { registerAuditCommands } from './commands/audit';
import { registerDebugCommand } from './commands/debug';

const program = new Command();

program
    .name('erst')
    .description('Error Recovery and Simulation Tool (ERST) for Stellar')
    .version('1.0.0');

// Register commands
registerProtocolCommands(program);
registerDebugCommand(program);

// Register audit commands
registerAuditCommands(program);

program.parse(process.argv);

// If no arguments provided, show help
if (!process.argv.slice(2).length) {
    program.outputHelp();
}
