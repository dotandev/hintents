# Architecture Decision Records (ADRs)

This directory contains Architecture Decision Records (ADRs) for the Erst project. ADRs capture important architectural decisions and their rationale.

## ADR Format

Each ADR follows the standard format:
- **Status**: Accepted, Proposed, Deprecated, or Superseded
- **Context**: Problem statement and background
- **Decision**: The chosen solution with technical details
- **Rationale**: Why this decision was made over alternatives
- **Implementation**: How the decision is implemented
- **Consequences**: Effects of this decision

## ADR Index

| ADR | Title | Status | Date |
|-----|-------|--------|------|
| [001](001-snapshot-caching-strategy.md) | Snapshot Caching Strategy using Bincode and SHA256 | Accepted | 2026-02-26 |
| [002](002-hsm-integration.md) | HSM Integration for Cryptographic Operations | Accepted | 2026-02-26 |

## ADR Process

### Proposing an ADR

1. Create a new ADR file using the next sequential number
2. Set status to "Proposed"
3. Submit a pull request for review
4. Address feedback and revise as needed

### Accepting an ADR

1. ADR must be reviewed by at least one maintainer
2. Consensus should be reached among the team
3. Update status to "Accepted"
4. Implement the decision if not already done

### Modifying an ADR

1. If an ADR needs changes, create a new ADR that supersedes it
2. Update the original ADR's status to "Superseded"
3. Reference the new ADR in the original

## ADR Template

```markdown
# ADR-XXX: [Title]

## Status
[Proposed | Accepted | Deprecated | Superseded]

## Context
[Problem statement and background]

## Decision
[The chosen solution]

## Rationale
[Why this decision was made]

## Consequences
[Effects of implementing this decision]
```

## Related Documentation

- [Architecture Overview](../ARCHITECTURE.md)
- [Contributing Guidelines](../CONTRIBUTING.md)
- [Technical Specifications](../README.md)
