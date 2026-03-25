# PR Title
feat(simulator): include optional linear memory dumps in exported state snapshots

# PR Description
## Summary
- Added optional `--include-memory` support to `erst export --snapshot` so state snapshots can include a base64-encoded Wasm linear memory dump when present in simulator output.
- Extended snapshot SDK structures to persist `linearMemory` alongside `ledgerEntries` and added decode helpers.
- Added `erst export decode-memory` CLI utility to decode and print human-readable memory segments (hex + ASCII) with `--offset` and `--length` controls.
- Added protocol field plumbing for `linear_memory_dump` in simulator response types.
- Added unit tests for memory snapshot encode/decode and persistence behavior.

## Why
This enables deeper post-failure debugging by preserving memory state in snapshots and making that data inspectable from CLI.

## Testing
- `go test ./internal/snapshot`
- `go test ./internal/cmd -run TestNonExistent -count=1`
- `cargo check --manifest-path simulator/Cargo.toml`

## Attachment (Proof)
<!-- Attach screenshot or artifact here -->
![Attach proof image here](ATTACH_IMAGE_HERE)

## How to add the attachment
1. Upload your screenshot/image to the GitHub PR comment box or PR description using drag-and-drop.
2. Copy the generated Markdown image URL from GitHub.
3. Replace `ATTACH_IMAGE_HERE` above with that URL.
4. Keep this section in the PR body so reviewers can quickly verify behavior.
