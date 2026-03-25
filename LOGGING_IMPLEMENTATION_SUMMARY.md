# Implementation Summary: Unified Logging System

## Issue

#194 - Add --log-level=debug|trace spanning both Go and Rust

## Objective

Funnel Rust env_logger streams directly into Go's global logger to present interleaved debugging.

## Implementation Details

### Files Modified

1. **internal/logger/logger.go**
   - Added `LevelTrace` constant for trace-level logging (more verbose than debug)
   - Enhanced `ParseLevel()` function to support "trace" level
   - Added `Trace()` helper function for trace-level logging
   - Added `GetRustLogLevel()` to map Go log levels to Rust env_logger levels
   - Added `GetRustLogFormat()` to determine JSON vs text format
   - Supports both environment variable and CLI flag configuration

2. **internal/logger/logger_test.go**
   - Added tests for trace level parsing and filtering
   - Added tests for `ParseLevel()` function with all levels
   - Added tests for `GetRustLogLevel()` mapping
   - Added tests for `GetRustLogFormat()` configuration
   - Added comprehensive log level hierarchy tests
   - Ensures trace logs are properly filtered at higher log levels

3. **internal/cmd/root.go**
   - Added `LogLevelFlag` global variable
   - Added `--log-level` persistent flag to root command
   - Integrated logger configuration in `PersistentPreRunE`
   - Flag accepts: trace, debug, info, warn, error
   - Applied to all subcommands automatically

4. **internal/simulator/runner.go**
   - Modified `Run()` method to configure Rust logger via environment variables
   - Sets `RUST_LOG` based on Go log level
   - Sets `ERST_LOG_FORMAT` for format consistency
   - Captures Rust stderr output using `StderrPipe()`
   - Forwards Rust logs to Go logger in real-time with `[rust]` prefix
   - Uses `bufio.Scanner` for line-by-line log processing
   - Maintains chronological interleaving of Go and Rust logs

5. **Makefile**
   - Resolved merge conflicts in `.PHONY` declarations
   - Consolidated all phony targets into organized groups
   - Added `fmt`, `fmt-go`, `fmt-rust`, and `pre-commit` targets
   - Maintained docker targets from previous implementation

6. **README.md**
   - Added logging documentation link in Documentation section
   - Positioned between Architecture and Docker docs

### Files Created

1. **docs/LOGGING.md**
   - Comprehensive logging documentation (400+ lines)
   - Log level descriptions and usage examples
   - Command-line flag and environment variable usage
   - Text and JSON output format examples
   - Rust integration explanation and log mapping table
   - Structured logging examples
   - Best practices for development, CI/CD, and production
   - Filtering and performance considerations
   - Troubleshooting guide
   - Integration examples (Docker, Systemd, Kubernetes)
   - API reference for both Go and Rust
   - Future enhancement ideas

## Key Features

### Log Levels

Supported levels (most to least verbose):

1. **trace** - Most verbose, all internal operations
2. **debug** - Detailed debugging information
3. **info** - General informational messages (default)
4. **warn** - Warning messages
5. **error** - Error messages only

### Unified Logging

- Single `--log-level` flag controls both Go and Rust components
- Rust logs automatically forwarded to Go logger
- Logs interleaved chronologically
- Rust logs prefixed with `[rust]` for identification
- Consistent formatting across both languages

### Log Level Mapping

| Go Level | Rust Level |
| -------- | ---------- |
| trace    | trace      |
| debug    | debug      |
| info     | info       |
| warn     | warn       |
| error    | error      |

### Output Formats

- **Text** (default): Human-readable with timestamps and structured fields
- **JSON**: Machine-parsable for log aggregation systems

## Usage Examples

### Basic Usage

```bash
# Trace level (most verbose)
erst --log-level=trace debug <tx-hash>

# Debug level
erst --log-level=debug debug <tx-hash>

# Info level (default)
erst debug <tx-hash>

# Warn level
erst --log-level=warn debug <tx-hash>

# Error level only
erst --log-level=error debug <tx-hash>
```

### Environment Variable

```bash
export ERST_LOG_LEVEL=debug
erst debug <tx-hash>
```

### JSON Format

```bash
export ERST_LOG_FORMAT=json
erst --log-level=debug debug <tx-hash>
```

### Example Output

```
level=DEBUG msg="Simulator binary resolved" path=/app/erst-sim source="dev target"
level=INFO msg="Starting simulation" tx_hash=abc123...def
level=INFO msg="[rust] Simulator initializing..." source=simulator
level=DEBUG msg="[rust] Loaded 15 Ledger Entries" source=simulator
level=INFO msg="[rust] CPU Instructions Used: 45000" source=simulator
level=INFO msg="Simulation completed" status=success duration_ms=123
```

## Technical Implementation

### Go Logger Enhancement

- Uses Go's `log/slog` package for structured logging
- Custom `LevelTrace` constant at level -8 (below Debug at -4)
- Thread-safe level management with mutex
- Dynamic level changes without restart
- Structured attributes for rich context

### Rust Log Integration

1. **Environment Configuration**: Go sets `RUST_LOG` and `ERST_LOG_FORMAT` before spawning Rust process
2. **Stderr Capture**: Uses `cmd.StderrPipe()` to capture Rust output
3. **Real-time Forwarding**: Goroutine reads stderr line-by-line with `bufio.Scanner`
4. **Prefix Addition**: Adds `[rust]` prefix to distinguish Rust logs
5. **Chronological Order**: Logs appear in order of occurrence

### Rust Simulator Changes

The Rust simulator already had logging infrastructure:

- Uses `tracing` and `tracing-subscriber` crates
- Respects `RUST_LOG` environment variable
- Supports JSON and text formats via `ERST_LOG_FORMAT`
- Writes logs to stderr (captured by Go)

## Testing

### Test Coverage

- **logger_test.go**: 15 test functions
  - Level parsing (trace, debug, info, warn, error)
  - Level filtering and hierarchy
  - Trace logging functionality
  - Rust log level mapping
  - Format configuration
  - Concurrent logging
  - JSON and text output

### Running Tests

```bash
# Run logger tests
go test -v ./internal/logger

# Run all tests
go test -v ./...

# Test with different log levels
ERST_LOG_LEVEL=trace go test -v ./internal/simulator
ERST_LOG_LEVEL=debug go test -v ./internal/simulator
```

## Verification

The implementation can be verified by:

1. **CLI Flag Test**

   ```bash
   erst --log-level=debug --help
   erst --log-level=trace debug <tx-hash>
   ```

2. **Environment Variable Test**

   ```bash
   export ERST_LOG_LEVEL=debug
   erst debug <tx-hash>
   ```

3. **Rust Log Integration Test**

   ```bash
   # Should see [rust] prefixed logs
   erst --log-level=debug debug <tx-hash> 2>&1 | grep '\[rust\]'
   ```

4. **Level Filtering Test**

   ```bash
   # Trace should show everything
   erst --log-level=trace debug <tx-hash>

   # Error should show only errors
   erst --log-level=error debug <tx-hash>
   ```

5. **JSON Format Test**
   ```bash
   export ERST_LOG_FORMAT=json
   erst --log-level=debug debug <tx-hash>
   ```

## Benefits

1. **Unified Debugging**: Single flag controls both Go and Rust verbosity
2. **Interleaved Logs**: See Go and Rust operations in chronological order
3. **Easy Troubleshooting**: Trace entire execution flow across languages
4. **Production Ready**: Minimal overhead at info/warn/error levels
5. **Flexible Output**: Text for humans, JSON for machines
6. **Structured Context**: Rich metadata in every log entry
7. **No Code Changes**: Existing Rust logging works automatically

## Performance Impact

- **trace**: Significant overhead, development only
- **debug**: Moderate overhead, acceptable for debugging
- **info**: Minimal overhead, suitable for production
- **warn/error**: Negligible overhead

## Compliance

- No lints suppressed
- All code follows project conventions
- Comprehensive test coverage (15 test functions)
- Complete documentation (400+ lines)
- Clean commit history
- Backward compatible (default level unchanged)

## Branch

`feat/cli-issue-194`

## Commit History

```
e6da121 docs: add logging documentation to README
66a3feb chore(makefile): add formatting and pre-commit targets
ad19ab5 chore(makefile): consolidate and organize phony targets
17d47a1 feat(logging): add comprehensive logging documentation
a2d18be feat(logging): add log level configuration and Rust log forwarding
```

## Next Steps

1. **Push Branch**

   ```bash
   git push origin feat/cli-issue-194
   ```

2. **Create Pull Request**
   - Title: "Add --log-level flag spanning both Go and Rust"
   - Reference issue #194
   - Include usage examples

3. **Verify CI**
   - All tests pass
   - No lint errors
   - Documentation builds correctly

4. **Post-Merge**
   - Users can use `--log-level` flag immediately
   - Update user documentation with examples
   - Consider adding to quick start guide

## Future Enhancements

Potential improvements for future iterations:

- Colored output for terminal display
- Log rotation for long-running processes
- Log sampling for high-volume scenarios
- Distributed tracing integration (OpenTelemetry)
- Performance metrics in logs
- Custom log formatters
- Log level per component (e.g., `--log-level=simulator:trace,cli:info`)
- Real-time log streaming to external systems
- Log aggregation and search UI

## Integration Examples

### Docker

```bash
docker run --rm ghcr.io/dotandev/hintents:latest \
  --log-level=debug debug <tx-hash>
```

### CI/CD

```yaml
- name: Debug transaction
  run: |
    erst --log-level=info debug $TX_HASH
```

### Kubernetes

```yaml
env:
  - name: ERST_LOG_LEVEL
    value: "info"
  - name: ERST_LOG_FORMAT
    value: "json"
```

## Documentation

- **docs/LOGGING.md**: Complete logging guide
- **README.md**: Quick reference link
- **Code comments**: Inline documentation
- **Test examples**: Usage patterns

## Resolved Issues

- Merge conflict in Makefile resolved
- `.PHONY` declarations consolidated
- Formatting targets added
- All tests passing
- Documentation complete
