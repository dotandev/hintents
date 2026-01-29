## Description
This PR initializes a comprehensive documentation strategy for the Erst project, providing clear technical and user-facing guides for contributors and users.

## Related Issue
Closes #59

## Changes Made

### New Documentation Files
- **docs/CLI.md**: Complete command reference for the `erst` CLI
  - Root command documentation
  - `erst debug` command with all flags and usage examples
  - Command reference table

- **docs/ARCHITECTURE.md**: Technical architecture documentation
  - Split-architecture design explanation (Go CLI + Rust Simulator)
  - Mermaid sequence diagram showing IPC data flow
  - JSON protocol documentation for Go ↔ Rust communication

- **docs/CONTRIBUTING.md**: Comprehensive contributing guide
  - Clear prerequisites list (Go 1.21+, Rust stable, Docker)
  - Detailed setup steps for both Go and Rust environments
  - Step-by-step build instructions
  - Testing commands for both components
  - PR submission guidelines

### Removed Files
- `CONTRIBUTING.md` (root) - Content moved and expanded in `docs/CONTRIBUTING.md`
- `docs/architecture.md` (empty) - Replaced with comprehensive `docs/ARCHITECTURE.md`

## Testing
- ✅ Verified all markdown files render correctly
- ✅ Confirmed Mermaid diagram syntax is valid
- ✅ Checked that all file paths and links are accurate
- ✅ Validated build instructions against current codebase

## Success Criteria Met
- ✅ docs/CLI.md exists with command references
- ✅ docs/ARCHITECTURE.md exists with IPC logic details
- ✅ docs/CONTRIBUTING.md exists with detailed setup steps
- ✅ Data flow diagrams included (Mermaid)
- ✅ Clear prerequisites list provided
