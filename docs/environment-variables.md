# Environment Variables Reference

This document provides a comprehensive reference for all environment variables used by Erst.

## Configuration Variables

| Variable Name | Category | Description | Default Value | Example |
|---------------|----------|-------------|---------------|---------|
| `ERST_SIMULATOR_PATH` | Simulator | Custom path to the `erst-sim` binary. If not set, the system will search in common locations (current directory, development path, and system PATH). | *(auto-detected)* | `/usr/local/bin/erst-sim` |

## Variable Search Order

When `ERST_SIMULATOR_PATH` is not set, the system searches for the simulator binary in the following order:

1. **Environment Variable**: `ERST_SIMULATOR_PATH` (if set)
2. **Current Directory**: `./erst-sim`
3. **Development Path**: `./simulator/target/release/erst-sim`
4. **System PATH**: Any `erst-sim` binary in your system PATH

## Usage Examples

### Setting Environment Variables

**Linux/macOS:**
```bash
export ERST_SIMULATOR_PATH="/path/to/custom/erst-sim"
./erst debug <transaction-hash>
```

**Windows (PowerShell):**
```powershell
$env:ERST_SIMULATOR_PATH = "C:\path\to\custom\erst-sim.exe"
.\erst debug <transaction-hash>
```

**Docker:**
```dockerfile
ENV ERST_SIMULATOR_PATH=/usr/local/bin/erst-sim
```

### Temporary Override
```bash
ERST_SIMULATOR_PATH="/tmp/debug-sim" ./erst debug abc123...
```

## Notes

- All environment variables are optional and have sensible defaults
- The simulator binary path detection is designed to work out-of-the-box for development and production environments
- If the simulator binary cannot be found in any location, Erst will display a helpful error message with setup instructions
