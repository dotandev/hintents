import * as os from 'os';
import * as path from 'path';
import * as fs from 'fs/promises';
import { exec } from 'child_process';
import { promisify } from 'util';

const execAsync = promisify(exec);

/**
 * ProtocolRegistrar handles the registration and unregistration of the
 * custom URI protocol handler (erst://) across different operating systems.
 */
export class ProtocolRegistrar {
    private readonly protocol = 'erst';
    private readonly cliPath: string;

    constructor() {
        // Get the absolute path to the ERST CLI executable
        // In production, this would be the actual binary path
        this.cliPath = process.execPath;
    }

    /**
     * Register the erst:// protocol handler for the current OS
     */
    async register(): Promise<void> {
        const platform = os.platform();

        try {
            switch (platform) {
                case 'win32':
                    await this.registerWindows();
                    break;
                case 'darwin':
                    await this.registerMacOS();
                    break;
                case 'linux':
                    await this.registerLinux();
                    break;
                default:
                    throw new Error(`Unsupported platform: ${platform}`);
            }

            console.log(`✅ Protocol handler registered for ${this.protocol}://`);
        } catch (error) {
            console.error('Failed to register protocol handler:', error);
            throw error;
        }
    }

    /**
     * Windows: Register via Registry
     */
    private async registerWindows(): Promise<void> {
        const regPath = `HKEY_CURRENT_USER\\Software\\Classes\\${this.protocol}`;

        const commands = [
            `reg add "${regPath}" /ve /d "URL:ERST Protocol" /f`,
            `reg add "${regPath}" /v "URL Protocol" /d "" /f`,
            `reg add "${regPath}\\shell\\open\\command" /ve /d "\\"${this.cliPath}\\" protocol-handler \\"%1\\"" /f`,
        ];

        for (const cmd of commands) {
            await execAsync(cmd);
        }
    }

    /**
     * macOS: Register via Info.plist
     */
    private async registerMacOS(): Promise<void> {
        // Create a LaunchAgent plist file
        const plistPath = path.join(
            os.homedir(),
            'Library',
            'LaunchAgents',
            `com.erst.protocol.plist`,
        );

        const plistContent = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.erst.protocol</string>
    <key>CFBundleURLTypes</key>
    <array>
        <dict>
            <key>CFBundleURLName</key>
            <string>ERST Protocol</string>
            <key>CFBundleURLSchemes</key>
            <array>
                <string>${this.protocol}</string>
            </array>
        </dict>
    </array>
    <key>ProgramArguments</key>
    <array>
        <string>${this.cliPath}</string>
        <string>protocol-handler</string>
    </array>
    <key>StandardInPath</key>
    <string>/dev/null</string>
    <key>StandardOutPath</key>
    <string>/tmp/erst-protocol.log</string>
    <key>StandardErrorPath</key>
    <string>/tmp/erst-protocol-error.log</string>
</dict>
</plist>`;

        await fs.writeFile(plistPath, plistContent, 'utf8');
        await execAsync(`launchctl load ${plistPath}`);
    }

    /**
     * Linux: Register via .desktop file
     */
    private async registerLinux(): Promise<void> {
        const desktopPath = path.join(
            os.homedir(),
            '.local',
            'share',
            'applications',
            'erst-protocol.desktop',
        );

        const desktopContent = `[Desktop Entry]
Version=1.0
Type=Application
Name=ERST Protocol Handler
Exec=${this.cliPath} protocol-handler %u
MimeType=x-scheme-handler/${this.protocol};
NoDisplay=true
Terminal=false`;

        // Ensure directory exists
        await fs.mkdir(path.dirname(desktopPath), { recursive: true });
        await fs.writeFile(desktopPath, desktopContent, 'utf8');

        // Register MIME type
        await execAsync(`xdg-mime default erst-protocol.desktop x-scheme-handler/${this.protocol}`);
        await execAsync('update-desktop-database ~/.local/share/applications/');
    }

    /**
     * Unregister protocol handler
     */
    async unregister(): Promise<void> {
        const platform = os.platform();

        try {
            switch (platform) {
                case 'win32':
                    await execAsync(`reg delete "HKEY_CURRENT_USER\\Software\\Classes\\${this.protocol}" /f`);
                    break;
                case 'darwin':
                    const plistPath = path.join(os.homedir(), 'Library', 'LaunchAgents', 'com.erst.protocol.plist');
                    await execAsync(`launchctl unload ${plistPath}`);
                    await fs.unlink(plistPath);
                    break;
                case 'linux':
                    const desktopPath = path.join(os.homedir(), '.local', 'share', 'applications', 'erst-protocol.desktop');
                    await fs.unlink(desktopPath);
                    break;
            }

            console.log('✅ Protocol handler unregistered');
        } catch (error) {
            console.error('Failed to unregister protocol handler:', error);
        }
    }

    /**
     * Check if protocol is already registered
     */
    async isRegistered(): Promise<boolean> {
        const platform = os.platform();

        try {
            switch (platform) {
                case 'win32':
                    const { stdout } = await execAsync(`reg query "HKEY_CURRENT_USER\\Software\\Classes\\${this.protocol}"`);
                    return stdout.includes('URL Protocol');
                case 'darwin':
                    const plistPath = path.join(os.homedir(), 'Library', 'LaunchAgents', 'com.erst.protocol.plist');
                    await fs.access(plistPath);
                    return true;
                case 'linux':
                    const desktopPath = path.join(os.homedir(), '.local', 'share', 'applications', 'erst-protocol.desktop');
                    await fs.access(desktopPath);
                    return true;
                default:
                    return false;
            }
        } catch {
            return false;
        }
    }
}
