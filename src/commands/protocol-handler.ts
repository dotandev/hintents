import { Command } from 'commander';
import { ProtocolHandler } from '../protocol/handler';
import { ProtocolRegistrar } from '../protocol/register';
import * as dotenv from 'dotenv';

// Load environment variables for security configuration
dotenv.config();

/**
 * registerProtocolCommands adds protocol-related commands to the ERST CLI.
 * These include the internal handler called by the OS and user-facing 
 * registration commands.
 */
export function registerProtocolCommands(program: Command): void {
    // 1. Internal Protocol Handler (hidden from standard help via description)
    // TODO: Implement a lock mechanism if multiple instances are launched simultaneously
    program
        .command('protocol-handler <uri>')
        .description('Internal: Handle ERST protocol URI (invoked by OS)')
        .action(async (uri: string) => {
            try {
                const handler = new ProtocolHandler({
                    secret: process.env.ERST_PROTOCOL_SECRET,
                    trustedOrigins: process.env.ERST_TRUSTED_ORIGINS?.split(','),
                });

                await handler.handle(uri);
            } catch (error) {
                if (error instanceof Error) {
                    console.error(`❌ Protocol handler error: ${error.message}`);
                } else {
                    console.error('❌ Protocol handler error: An unknown error occurred');
                }
                process.exit(1);
            }
        });

    // 2. Protocol Registration
    program
        .command('protocol:register')
        .description('Register the erst:// protocol handler in the operating system')
        .action(async () => {
            try {
                const registrar = new ProtocolRegistrar();

                const isRegistered = await registrar.isRegistered();
                if (isRegistered) {
                    console.log('⚠️  Protocol handler is already registered.');
                    console.log('To refresh registration, run: erst protocol:unregister && erst protocol:register');
                    return;
                }

                await registrar.register();
                console.log('✅ Successfully registered ERST protocol handler');
                console.log('You can now launch ERST directly from supported dashboards via erst:// links.');
            } catch (error) {
                if (error instanceof Error) {
                    console.error(`❌ Registration failed: ${error.message}`);
                } else {
                    console.error('❌ Registration failed: An unknown error occurred');
                }
                process.exit(1);
            }
        });

    // 3. Protocol Unregistration
    program
        .command('protocol:unregister')
        .description('Unregister the erst:// protocol handler from the operating system')
        .action(async () => {
            try {
                const registrar = new ProtocolRegistrar();
                await registrar.unregister();
                console.log('✅ Successfully unregistered ERST protocol handler');
            } catch (error) {
                if (error instanceof Error) {
                    console.error(`❌ Unregistration failed: ${error.message}`);
                } else {
                    console.error('❌ Unregistration failed: An unknown error occurred');
                }
                process.exit(1);
            }
        });

    // 4. Registration Status
    program
        .command('protocol:status')
        .description('Check current registration status of the erst:// protocol handler')
        .action(async () => {
            try {
                const registrar = new ProtocolRegistrar();
                const isRegistered = await registrar.isRegistered();

                if (isRegistered) {
                    console.log('✅ ERST protocol handler is currently REGISTERED');
                } else {
                    console.log('❌ ERST protocol handler is NOT REGISTERED');
                    console.log('Run "erst protocol:register" to enable dashboard integration.');
                }
            } catch (error) {
                if (error instanceof Error) {
                    console.error(`❌ Status check failed: ${error.message}`);
                } else {
                    console.error('❌ Status check failed: An unknown error occurred');
                }
                process.exit(1);
            }
        });
}
