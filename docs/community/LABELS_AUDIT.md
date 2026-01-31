# Good First Issue – Labels Audit

**Branch:** `community/labels-audit`  
**Goal:** Lower the entry barrier for new contributors by labeling easy, newcomer-friendly tasks.

## Criteria

Tasks chosen for **good first issue**:

- Do **not** require deep Soroban-internals knowledge.
- Are scoped (e.g. docs, help text, UX, tests, tooling).
- Examples: help text improvements, doc updates, Docker/UX, small tests, QA tooling.

## List of Updated Issues

| #   | Title                                                                 | Rationale |
|-----|-----------------------------------------------------------------------|-----------|
| 32  | Source ID/Link                                                        | Documentation; already had label. |
| 81  | 29. [Decoder] Implement Trace visualization in CLI                    | Scoped decoder/CLI work; no Soroban internals. |
| 84  | 32. [Decoder] Suggestion engine for common errors                     | Help text / UX; error messages. |
| 86  | 34. [UX] Create Dockerfile for reproducible environment               | DevOps/UX; no core logic. |
| 87  | 35. [UX] Add doctor command for environment verification             | UX/tooling; env checks. |
| 114 | 62. [QA] Implement static analysis for Rust simulator                 | Tooling/QA; script or config. |
| 116 | 64. [QA] Implement dead code detection for Go codebase                | Tooling/QA; script or linter. |
| 130 | 78. [Docs] Add "Cookbook" for common debugging scenarios              | Documentation. |
| 131 | 79. [Community] Add "Good First Issue" labels                        | Meta community task. |
| 162 | 59. [Test] Add unit tests for ledger entry hashing                    | Table-driven tests; no Soroban internals. |

**Total:** 10 issues labeled (or to be labeled) with **good first issue**.

## Verification

- Filter by label [good first issue](https://github.com/dotandev/hintents/issues?q=is%3Aissue+is%3Aopen+label%3A%22good+first+issue%22) and confirm count ≥ 10.
- This PR does not change application code; only labels (and this doc + script) are added.

## Applying the labels

- **Option A (UI):** Open each issue above and add the label **good first issue** in the GitHub UI.
- **Option B (script):** From repo root, with a `GITHUB_TOKEN` that has `issues: write`:

  ```bash
  export GITHUB_TOKEN="your_token"
  ./scripts/label-good-first-issue.sh
  ```

  Script repo default: `dotandev/hintents`. Override with `GITHUB_REPO=owner/repo` if needed.
