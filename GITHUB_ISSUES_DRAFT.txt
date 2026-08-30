# 🚀 ERST SDK: 20 Micro-Improvements (SCF Ready)

These are small, targeted issues to refine the SDK's stability and user experience, making it "Institutional Grade" for the SCF application.

---

### 🛡️ Security & Resilience
1. **[Security] HSM PIN Redaction in Logs**: Implement a utility in `internal/logger` to scrub `ERST_PKCS11_PIN` values from error messages and debug logs.
2. **[Security] Add `gosec` to CI**: Add a GitHub Action step to run `gosec` on the Go codebase to catch common security anti-patterns.
3. **[Stability] Add Build Timeout to Doctor**: Wrap the `FixSimulatorBinary` build command in a 5-minute context timeout to prevent hung builds from blocking the CLI.
4. **[Fix] Windows Path Normalization Audit**: Audit `internal/config/config.go` to ensure all `CachePath` calculations use `filepath.ToSlash` for registry/config storage consistency.
5. **[Git] Windows Build Artifacts in .gitignore**: Add `*.exe`, `*.pdb`, and `target/` (recursive) to the root `.gitignore` to prevent committing Windows binary noise.

### 🔧 Developer Experience (DX)
6. **[Doctor] Functional Version Check**: Update `doctor` to run `./erst-sim --version` instead of just checking if the file exists.
7. **[Doctor] Colorized "Fixable" Status**: Update the `doctor` output so that fixable failures (`[FAIL*]`) are Yellow, distinguishing them from fatal errors.
8. **[CLI] Add `--version` to all Subcommands**: Ensure `erst debug --version` and `erst session --version` return the same consistent version string as the root.
9. **[Version] Centralize SDK Version**: Move the hardcoded version in `internal/cmd/root.go` to a dedicated `internal/version/version.go` package.
10. **[UX] Shorten Registry Paths**: In `doctor` output, truncate absolute home paths to `~/.erst` to improve readability on narrow terminals.

### 🧪 Testing & Reliability
11. **[Test] Unit Test for `buildDeepLinkFixHint`**: Add edge-case testing for the deep link hint builder in `internal/cmd/doctor_test.go`.
12. **[CI] Jitter-Aware Performance Mean**: Update the perf test to run 3 times and use the mean, allowing us to lower the 10000% noise threshold.
13. **[Test] Mock HSM for Integration Tests**: Create a minimal mock PKCS#11 module (using `libloading`) for testing HSM logic without physical hardware.
14. **[Lint] Standardize `nolint` Reasons**: Systematically add descriptive reasons to all `//nolint` directives in the codebase (e.g., `//nolint:gosec // binary builder`).

### 📦 Logic & Cleanup
15. **[Style] Constantize Cache Subdirs**: Move `"transactions"`, `"protocols"`, and `"contracts"` strings to package-level constants in `internal/cmd`.
16. **[Cleanup] Remove Legacy TODOs**: Address the "Use d.Runner" TODO in `internal/cmd/debug.go` now that the runner is stable.
17. **[RPC] Add User-Agent Header**: Add an `ERST-SDK/v0.1.0` User-Agent to all RPC requests to help Stellar node operators identify traffic.
18. **[Config] Validate `MaxTraceDepth`**: Add a validation check in `internal/config` to ensure `MaxTraceDepth` is a positive integer between 1 and 1000.
19. **[Logic] Detect Protocol Registry Conflicts**: Update `protocol:register` to warn if the `erst://` scheme is already claimed by another binary in the Windows Registry.
20. **[UX] Success Banner for `init`**: Add a professional success banner (colorized) when `erst init` completes, listing the next 3 logical steps for the user.
