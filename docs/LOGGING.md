# Unified Logging System

## Overview

The `erst` tool provides a unified logging system that spans both Go and Rust components. The `--log-level` flag controls verbosity for both the Go CLI and the Rust simulator, with logs interleaved in a single output stream.

## Log Levels

The following log levels are supported (from most to least verbose):

1. **trace** - Most verbose, includes all internal operations
2. **debug** - Detailed debugging information
3. **info** - General informational messages (default)
4. **warn** - Warning messages for potentially problematic situations
5. **error** - Error messages for failures

## Usage

### Command Line Flag

Use the `--log-level` flag to set the logging verbosity:

```bash
# Trace level (most verbose)
erst --log-level=trace debug <tx-hash>

# Debug level
erst --log-level=debug debug <tx-hash>

# Info level (default)
erst --log-level=info debug <tx-hash>

# Warn level
erst --log-level=warn debug <tx-hash>

# Error level (least verbose)
erst --log-level=error debug <tx-hash>
```

### Environment Variable

Alternatively, set the log level via environment variable:

```bash
export ERST_LOG_LEVEL=debug
erst debug <tx-hash>
```

The command-line flag takes precedence over the environment variable.

## Log Output Format

### Text Format (Default)

Human-readable text output with timestamps and structured fields:

```
time=2026-02-24T10:30:45.123Z level=INFO msg="Starting simulation" tx_hash=abc123
time=2026-02-24T10:30:45.456Z level=DEBUG msg="Simulator binary resolved" path=/app/erst-sim source="dev target"
time=2026-02-24T10:30:45.789Z level=INFO msg="[rust] Simulator initializing..." source=simulator
time=2026-02-24T10:30:46.012Z level=INFO msg="[rust] Host Initialized with Budget" source=simulator
```

### JSON Format

Machine-parsable JSON output for log aggregation systems:

```bash
export ERST_LOG_FORMAT=json
erst --log-level=debug debug <tx-hash>
```

Output:

```json
{"time":"2026-02-24T10:30:45.123Z","level":"INFO","msg":"Starting simulation","tx_hash":"abc123"}
{"time":"2026-02-24T10:30:45.456Z","level":"DEBUG","msg":"Simulator binary resolved","path":"/app/erst-sim","source":"dev target"}
```

## Rust Integration

### How It Works

The Go CLI automatically configures the Rust simulator's logging by:

1. Setting the `RUST_LOG` environment variable based on `--log-level`
2. Setting the `ERST_LOG_FORMAT` environment variable for format consistency
3. Capturing the Rust simulator's stderr output
4. Forwarding Rust logs to the Go logger with a `[rust]` prefix
5. Interleaving Go and Rust logs in chronological order

### Rust Log Mapping

Go log levels are mapped to Rust `env_logger` levels:

| Go Level | Rust Level |
| -------- | ---------- |
| trace    | trace      |
| debug    | debug      |
| info     | info       |
| warn     | warn       |
| error    | error      |

### Identifying Rust Logs

Rust logs are prefixed with `[rust]` in the message:

```
level=INFO msg="[rust] Simulator initializing..." source=simulator
level=DEBUG msg="[rust] Loading ledger entries" source=simulator count=42
```

## Examples

### Basic Debugging

```bash
# See what's happening during simulation
erst --log-level=debug debug abc123...def
```

Output:

```
level=DEBUG msg="Simulator binary resolved" path=/app/erst-sim
level=INFO msg="Starting simulation" tx_hash=abc123...def
level=INFO msg="[rust] Simulator initializing..." source=simulator
level=DEBUG msg="[rust] Loaded 15 Ledger Entries" source=simulator
level=INFO msg="Simulation completed" status=success
```

### Trace Level for Deep Debugging

```bash
# See every internal operation
erst --log-level=trace debug abc123...def
```

This shows:

- All function calls and returns
- Data transformations
- Protocol configurations
- Budget calculations
- Every Rust operation

### Production Use (Minimal Logging)

```bash
# Only show errors
erst --log-level=error debug abc123...def
```

### JSON Logging for Monitoring

```bash
# Export logs to a monitoring system
export ERST_LOG_FORMAT=json
erst --log-level=info debug abc123...def | tee -a logs/erst.jsonl
```

## Structured Logging

All logs include structured fields for easy parsing and filtering:

```go
logger.Logger.Info("Simulation completed",
    "tx_hash", txHash,
    "status", "success",
    "duration_ms", duration,
    "cpu_instructions", cpuUsed,
)
```

Output:

```
level=INFO msg="Simulation completed" tx_hash=abc123 status=success duration_ms=123 cpu_instructions=45000
```

## Best Practices

### Security

**HSM PIN Redaction**: The logging system automatically redacts HSM PIN values from all log output to prevent sensitive credentials from appearing in logs. Any occurrence of `ERST_PKCS11_PIN` followed by a value will be replaced with `ERST_PKCS11_PIN=*****`. This applies to both text and JSON log formats.

Example:
```
# Before redaction
ERST_PKCS11_PIN=1234

# After redaction
ERST_PKCS11_PIN=*****
```

### Development

Use `debug` or `trace` level during development:

```bash
erst --log-level=debug debug <tx-hash>
```

### CI/CD

Use `info` level in CI/CD pipelines:

```bash
erst --log-level=info debug <tx-hash>
```

### Production

Use `warn` or `error` level in production:

```bash
erst --log-level=warn debug <tx-hash>
```

### Log Aggregation

Use JSON format with a log aggregation system:

```bash
export ERST_LOG_FORMAT=json
erst --log-level=info debug <tx-hash> 2>&1 | \
  jq -c '. + {service: "erst", environment: "production"}'
```

## Filtering Logs

### By Level

```bash
# Only show warnings and errors
erst --log-level=warn debug <tx-hash>
```

### By Source

```bash
# Filter for Rust logs only
erst --log-level=debug debug <tx-hash> 2>&1 | grep '\[rust\]'

# Filter for Go logs only
erst --log-level=debug debug <tx-hash> 2>&1 | grep -v '\[rust\]'
```

### By Field (JSON)

```bash
export ERST_LOG_FORMAT=json
erst --log-level=debug debug <tx-hash> 2>&1 | \
  jq 'select(.source == "simulator")'
```

## Performance Considerations

### Log Level Impact

- **trace**: Significant performance impact, use only for debugging
- **debug**: Moderate performance impact, acceptable for development
- **info**: Minimal performance impact, suitable for production
- **warn/error**: Negligible performance impact

### Buffering

Logs are written to stderr and are line-buffered by default. For high-throughput scenarios, consider redirecting to a file:

```bash
erst --log-level=info debug <tx-hash> 2>> erst.log
```

## Troubleshooting

### No Rust Logs Appearing

If Rust logs don't appear:

1. Check that the simulator binary is being executed
2. Verify `RUST_LOG` is set correctly (check with `--log-level=debug`)
3. Ensure stderr is not being redirected elsewhere

### Log Level Not Working

If log level changes don't take effect:

1. Ensure the flag is before the subcommand: `erst --log-level=debug debug <tx>`
2. Check for conflicting `ERST_LOG_LEVEL` environment variable
3. Verify the log level string is valid (trace, debug, info, warn, error)

### Logs Out of Order

Logs from Go and Rust may occasionally appear slightly out of order due to buffering. This is normal and doesn't affect functionality.

## Integration with Other Tools

### Docker

```bash
docker run --rm ghcr.io/dotandev/hintents:latest \
  --log-level=debug debug <tx-hash>
```

### Systemd

```ini
[Service]
Environment="ERST_LOG_LEVEL=info"
Environment="ERST_LOG_FORMAT=json"
StandardOutput=journal
StandardError=journal
```

### Kubernetes

```yaml
env:
  - name: ERST_LOG_LEVEL
    value: "info"
  - name: ERST_LOG_FORMAT
    value: "json"
```

## API Reference

### Go Logger Functions

```go
import "github.com/dotandev/hintents/internal/logger"

// Set log level
logger.SetLevel(logger.LevelTrace)
logger.SetLevel(slog.LevelDebug)
logger.SetLevel(slog.LevelInfo)

// Parse level from string
level := logger.ParseLevel("debug")

// Log at different levels
logger.Trace("trace message", "key", "value")
logger.Logger.Debug("debug message", "key", "value")
logger.Logger.Info("info message", "key", "value")
logger.Logger.Warn("warn message", "key", "value")
logger.Logger.Error("error message", "key", "value")

// Get Rust-compatible log level
rustLevel := logger.GetRustLogLevel() // Returns "debug", "info", etc.

// Redact sensitive PIN values from strings
redacted := logger.RedactPIN("ERST_PKCS11_PIN=1234") // Returns "ERST_PKCS11_PIN=*****"
```

### Rust Logger (env_logger)

The Rust simulator uses `tracing` and `tracing-subscriber`:

```rust
use tracing::{trace, debug, info, warn, error};

trace!("trace message");
debug!("debug message");
info!("info message");
warn!("warn message");
error!("error message");

// With structured fields
info!(event = "simulation_started", tx_hash = %hash);
```

## Future Enhancements

Potential improvements for future versions:

- Log rotation for long-running processes
- Colored output for terminal display
- Log sampling for high-volume scenarios
- Distributed tracing integration
- Performance metrics in logs
- Custom log formatters
