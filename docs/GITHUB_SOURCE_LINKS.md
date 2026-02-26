# GitHub Source Link Generation

This document describes the GitHub source link generation feature for the ERST simulator.

## Overview

When code resides in a Git repository with a GitHub remote, ERST automatically generates clickable GitHub links in traces that point directly to the source file and line number where errors or events occurred. This enables developers to quickly navigate from trace output to the exact source code location.

## Features

- Automatic Git repository detection
- GitHub URL generation with commit hash and line numbers
- Support for both SSH and HTTPS Git remotes
- Graceful fallback when Git repository is not available
- Integration with source mapping and debug symbols

## How It Works

### 1. Git Repository Detection

The simulator automatically detects if the code is in a Git repository by:
- Searching for `.git` directory starting from the current working directory
- Extracting the remote origin URL
- Getting the current commit hash
- Normalizing GitHub URLs (converts SSH to HTTPS format)

### 2. GitHub Link Generation

When source location information is available (from debug symbols), the system generates links in the format:

```
https://github.com/{owner}/{repo}/blob/{commit_hash}/{file_path}#L{line_number}
```

Example:
```
https://github.com/dotandev/hintents/blob/abc123def456/src/token.rs#L45
```

### 3. Trace Integration

GitHub links are included in:
- Trace JSON output (`github_link` field)
- Interactive trace viewer display
- Error messages and diagnostic output
- Webhook notifications (Slack, Discord)

## Usage

### In Trace Output

When viewing traces, source locations with GitHub links are displayed:

```
[FILE] Source: token.rs:45
[LINK] GitHub: https://github.com/dotandev/hintents/blob/abc123/src/token.rs#L45
```

### In JSON Output

```json
{
  "step": 5,
  "operation": "contract_call",
  "error": "Contract execution failed",
  "source_file": "token.rs",
  "source_line": 45,
  "github_link": "https://github.com/dotandev/hintents/blob/abc123/src/token.rs#L45"
}
```

### Interactive Viewer

The trace viewer automatically displays GitHub links when available:

```bash
./erst trace sample.json
```

Output:
```
[LOC] Current State
================
Step: 5/10
Operation: contract_call
Error: Contract execution failed
[FILE] Source: token.rs:45
[LINK] GitHub: https://github.com/dotandev/hintents/blob/abc123/src/token.rs#L45
```

## Requirements

### For GitHub Link Generation

1. Code must be in a Git repository
2. Repository must have a GitHub remote (origin)
3. Working directory must be within the repository
4. Git command-line tool must be available

### For Source Mapping

1. Contract WASM must include debug symbols
2. Debug symbols must contain file and line information
3. WASM must be provided in simulation request

## Configuration

No additional configuration is required. The feature works automatically when:
- Running in a Git repository with GitHub remote
- Debug symbols are present in WASM

## Architecture

### Components

1. **GitRepository** (`simulator/src/git_detector.rs`)
   - Detects Git repository
   - Extracts remote URL and commit hash
   - Generates GitHub links

2. **SourceMapper** (`simulator/src/source_mapper.rs`)
   - Maps WASM offsets to source locations
   - Integrates with GitRepository
   - Creates SourceLocation with GitHub links

3. **TraceNode** (`internal/trace/node.go`)
   - Stores source file, line, and GitHub link
   - Used in trace tree structure

4. **ExecutionState** (`internal/trace/navigation.go`)
   - Includes source location fields
   - Serialized in trace JSON

5. **InteractiveViewer** (`internal/trace/viewer.go`)
   - Displays GitHub links in terminal UI
   - Formats source location information

### Data Flow

```
WASM with Debug Symbols
    ↓
SourceMapper (detects debug symbols)
    ↓
GitRepository (detects repo, generates URL)
    ↓
SourceLocation (file, line, github_link)
    ↓
SimulationResponse (includes source_location)
    ↓
ExecutionState/TraceNode (stores in trace)
    ↓
InteractiveViewer (displays to user)
```

## Examples

### Example 1: Contract Error with Source Link

```bash
$ ./erst debug --generate-trace tx-hash-123

Error: Contract execution failed at token.rs:45
Source: https://github.com/myorg/mycontract/blob/abc123/src/token.rs#L45
```

### Example 2: Trace Navigation

```bash
$ ./erst trace trace.json

> j 5
[TARGET] Jumped to step 5

[LOC] Current State
================
Step: 5/10
Operation: contract_call
Function: transfer
Error: Insufficient balance
[FILE] Source: token.rs:45
[LINK] GitHub: https://github.com/myorg/mycontract/blob/abc123/src/token.rs#L45
```

### Example 3: JSON Output

```json
{
  "transaction_hash": "tx-hash-123",
  "states": [
    {
      "step": 5,
      "operation": "contract_call",
      "function": "transfer",
      "error": "Insufficient balance",
      "source_file": "token.rs",
      "source_line": 45,
      "github_link": "https://github.com/myorg/mycontract/blob/abc123/src/token.rs#L45"
    }
  ]
}
```

## Limitations

1. Only GitHub repositories are supported (not GitLab, Bitbucket, etc.)
2. Requires Git command-line tool to be installed
3. Links use commit hash, not branch name (ensures permanence)
4. Private repositories require user to be authenticated with GitHub
5. Source mapping requires debug symbols in WASM

## Future Enhancements

Potential improvements for future versions:

1. Support for other Git hosting platforms (GitLab, Bitbucket)
2. Configuration option to use branch name instead of commit hash
3. Deep linking to specific code ranges (not just single lines)
4. Integration with IDE protocols for direct file opening
5. Caching of Git repository information for performance

## Testing

### Unit Tests

Run the Git detector tests:
```bash
cd simulator
cargo test git_detector
```

Run the source mapper tests:
```bash
cd simulator
cargo test source_mapper
```

### Integration Tests

Test with a real contract:
```bash
# Build contract with debug symbols
cargo build --target wasm32-unknown-unknown --profile release-with-debug

# Run simulation
./erst debug --contract-wasm contract.wasm tx-hash-123
```

## Troubleshooting

### GitHub Links Not Generated

**Problem**: Traces don't include GitHub links

**Solutions**:
1. Verify you're in a Git repository: `git status`
2. Check remote URL: `git remote -v`
3. Ensure remote is GitHub: URL should contain `github.com`
4. Verify debug symbols in WASM: Check build configuration

### Invalid GitHub URLs

**Problem**: Generated URLs are incorrect

**Solutions**:
1. Check Git remote URL format
2. Verify commit hash: `git rev-parse HEAD`
3. Ensure file paths are relative to repository root

### Links to Private Repositories

**Problem**: GitHub returns 404 for private repo links

**Solutions**:
1. Ensure you're authenticated with GitHub
2. Verify repository access permissions
3. Consider using branch name for internal sharing

## Related Documentation

- [Source Mapping Implementation](source-mapping.md)
- [Trace Navigation](trace-navigation.md)
- [Debug Symbols Guide](debug-symbols-guide.md)
