#!/usr/bin/env node
import { Command } from 'commander';
import { registerProtocolCommands } from './commands/protocol-handler';

const program = new Command();

program
    .name('erst')
    .description('Error Recovery and Simulation Tool (ERST) for Stellar')
    .version('1.0.0');

// Register protocol-specific commands
registerProtocolCommands(program);

program.parse(process.argv);

// If no arguments provided, show help
if (!process.argv.slice(2).length) {
    program.outputHelp();
}
