# CLI Command Registration Architecture

This document explains the modular command registration system used in the `erst` CLI.

## Overview

The CLI uses a **registry pattern** to keep command registration clean, modular, and maintainable. This approach prevents `root.go` from becoming a "kitchen sink" of imports and configurations as we add more commands.

## Architecture

### File Structure

```
internal/cmd/
├── root.go          # Root command and central registration
├── debug.go         # Debug command implementation
├── version.go       # Version command implementation
└── [future].go      # Future commands...
```

### Key Components

#### 1. `root.go` - Central Registry

The root command file contains:
- Root command definition
- `RegisterCommands()` function - central registration point
- Utility functions shared across commands

**Key principle**: Commands are registered in **alphabetical order** to maintain consistent help output.

```go
func RegisterCommands(root *cobra.Command) {
    // Commands registered alphabetically
    registerDebugCommand(root)
    registerVersionCommand(root)
    // ... future commands
}
```

#### 2. Individual Command Files

Each command lives in its own file with three functions:

1. **`register[Command]Command(root *cobra.Command)`** - Registration function
   - Called from `RegisterCommands()` in `root.go`
   - Adds the command to the root

2. **`new[Command]Command() *cobra.Command`** - Command factory
   - Creates and configures the cobra.Command
   - Defines flags, usage, examples
   - Separates creation from registration for testability

3. **`run[Command](...) error`** - Command logic
   - Contains the actual implementation
   - Separated for easy unit testing
   - Returns errors for proper handling

### Example: Adding a New Command

To add a new command called `analyze`:

1. **Create `internal/cmd/analyze.go`**:

```go
package cmd

import (
    "github.com/spf13/cobra"
)

// registerAnalyzeCommand registers the analyze command with the root command.
func registerAnalyzeCommand(root *cobra.Command) {
    root.AddCommand(newAnalyzeCommand())
}

// newAnalyzeCommand creates and returns the analyze command.
func newAnalyzeCommand() *cobra.Command {
    cmd := &cobra.Command{
        Use:   "analyze <transaction-hash>",
        Short: "Analyze transaction patterns",
        RunE: func(cmd *cobra.Command, args []string) error {
            return runAnalyze(args[0])
        },
    }
    return cmd
}

// runAnalyze executes the analyze command logic.
func runAnalyze(txHash string) error {
    // Implementation here
    return nil
}
```

2. **Register in `root.go`** (in alphabetical order):

```go
func RegisterCommands(root *cobra.Command) {
    registerAnalyzeCommand(root)  // ← Add here
    registerDebugCommand(root)
    registerVersionCommand(root)
}
```

## Benefits

### 1. **Modularity**
- Each command is self-contained in its own file
- Easy to find and modify specific commands
- Clear separation of concerns

### 2. **Maintainability**
- `root.go` stays clean and focused
- No sprawling imports or configurations
- Alphabetical ordering is easy to maintain

### 3. **Testability**
- Command creation (`new*Command`) can be tested independently
- Command logic (`run*`) can be unit tested without Cobra
- Registration logic is simple and doesn't need testing

### 4. **Scalability**
- Adding new commands doesn't clutter existing files
- Pattern is consistent across all commands
- Easy for new contributors to follow

### 5. **Consistency**
- All commands follow the same pattern
- Help output maintains alphabetical order
- Predictable structure for developers

## Naming Conventions

| Function Type | Pattern | Example |
|--------------|---------|---------|
| Registration | `register[Command]Command` | `registerDebugCommand` |
| Factory | `new[Command]Command` | `newDebugCommand` |
| Logic | `run[Command]` | `runDebug` |
| File | `[command].go` | `debug.go` |

## Testing Strategy

### Unit Testing Commands

```go
func TestRunDebug(t *testing.T) {
    // Test the logic function directly
    err := runDebug("abc123", "testnet", false)
    // assertions...
}
```

### Integration Testing

```go
func TestDebugCommand(t *testing.T) {
    // Test the command creation
    cmd := newDebugCommand()
    assert.Equal(t, "debug", cmd.Use)
    // assertions...
}
```

## Migration Guide

If you have existing commands in `root.go`:

1. Extract each command to its own file
2. Create the three functions (register, new, run)
3. Update `RegisterCommands()` to call the register function
4. Maintain alphabetical order
5. Remove old code from `root.go`

## Future Enhancements

Potential improvements to this pattern:

- **Command groups**: Group related commands (e.g., `erst debug`, `erst debug trace`)
- **Plugin system**: Allow external commands to register themselves
- **Auto-registration**: Use init() functions for automatic registration
- **Command metadata**: Add tags, categories, or feature flags

## References

- [Cobra Documentation](https://github.com/spf13/cobra)
- [Go Project Layout](https://github.com/golang-standards/project-layout)
