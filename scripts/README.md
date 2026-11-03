# Scripts Directory

This directory contains utility scripts for testing, verification, and automation.

## Available Scripts

### `security/scan-diff.sh`

Scans the lines a change adds for patterns associated with malicious code:
credentials, key material reaching a logger or a socket, code evaluated at
runtime, requests to hosts this project does not normally talk to, and changes
to CI itself.

**Purpose**: this project merges a lot of community contributions, which is the
path an attacker would use. The scan does not replace review — it decides which
twenty lines of a large diff a reviewer should read first.

**Usage**:
```bash
# Against the default branch, the way CI runs it
scripts/security/scan-diff.sh

# Any range
scripts/security/scan-diff.sh main feature-branch
```

Findings are either **BLOCK** (unambiguous, fails the build) or **REVIEW**
(a heuristic that needs a human; annotated but not blocking). False positives
are waived per check and per file in `.github/security-allowlist.txt`.

Run by the **Malicious Code Scan** workflow.

### `security/check-dependency-changes.sh`

Inspects dependency manifest changes: `go.mod` replace directives, Cargo git
and out-of-tree path dependencies, npm lifecycle scripts and off-registry
packages, new `build.rs` files, and manifests edited without their lockfile.
Lists every newly added module so a reviewer sees the real cost of a change
without expanding a lockfile diff.

**Usage**:
```bash
scripts/security/check-dependency-changes.sh [base-ref] [head-ref]
```

Run by the **Supply Chain** workflow.

### `security/scan-installed-packages.sh`

Scans **installed** npm packages for the payloads used in registry
compromises — install hooks, request-capture endpoints, credential harvesting,
and the network-interception trick that rewrites wallet addresses in flight.

**Purpose**: covers the attack the diff scanner structurally cannot see. When a
maintainer account is phished and a malicious version is published, the payload
ships inside the package as an `api.js`, an `index.mjs` or a `bundle.js` and
arrives during `npm install`. None of it appears in any pull request here — a
version resolves somewhere new and the code is simply there.

Run it **after** installing with `--ignore-scripts`, so an install-time payload
is inert on disk rather than already executed.

**Usage**:
```bash
# Every directory with a package-lock.json
scripts/security/scan-installed-packages.sh

# One project
scripts/security/scan-installed-packages.sh erst-ui
```

Packages permitted to run a command on install are pinned in
`.github/expected-install-scripts.txt`; anything else that wants to fails the
build. Four legitimate packages are listed there today (`esbuild`, `fsevents`,
`pkcs11js`, `unrs-resolver`), so the check fires on a package *gaining* an
install script rather than on install scripts existing at all.

Run by the **Supply Chain** workflow, which installs with `--ignore-scripts`
first.

## Adding New Scripts

When adding new scripts to this directory:

1. Make them executable:
   ```bash
   chmod +x scripts/your_script.sh
   ```

2. Add a shebang line:
   ```bash
   #!/bin/bash
   ```

3. Add copyright header:
   ```bash
   # Copyright 2026 Erst Users
   # SPDX-License-Identifier: Apache-2.0
   ```

4. Update this README with usage instructions

5. Add error handling:
   ```bash
   set -e  # Exit on error
   ```

## Support

For issues with scripts:
1. Check prerequisites are installed
2. Verify authentication
3. Check script permissions (`chmod +x`)
4. Review error messages
5. Open an issue on GitHub if problems persist
