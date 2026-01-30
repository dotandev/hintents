# Stellar Dashboard Integration

## Overview

ERST (Error Recovery and Simulation Tool) supports one-click debugging from the Stellar Dashboard via a custom URI protocol handler (`erst://`). This allows developers to instantly launch a debug session for any transaction directly from their browser.

## Installation

### 1. Register the Protocol Handler

To enable the protocol handler, run the following command in your terminal:

```bash
erst protocol:register
```

This command registers `erst://` with your operating system (Windows, macOS, or Linux).

### 2. Verify Registration

You can check if the protocol is correctly registered by running:

```bash
erst protocol:status
```

## Usage

### From Stellar Dashboard

Once registered, you can click any debug link on the Stellar Dashboard. The link format is:

```
erst://debug/<transaction_hash>?network=<testnet|mainnet>&operation=<index>
```

- **transaction_hash** (required): The 64-character hex hash of the transaction.
- **network** (required): Either `testnet` or `mainnet`.
- **operation** (optional): The index of the specific operation to focus on.

### Manual Launch

You can also launch the protocol handler manually from your terminal for testing:

```bash
# macOS
open "erst://debug/a1b2c3d4...?network=testnet"

# Windows
start "erst://debug/a1b2c3d4...?network=testnet"

# Linux
xdg-open "erst://debug/a1b2c3d4...?network=testnet"
```

## Security

ERST implements several security layers to protect your environment:

### Signature Verification

To ensure that debug requests only come from trusted sources, you can enable HMAC-SHA256 signature verification.

1. Set your secret key:
   ```bash
   export ERST_PROTOCOL_SECRET=your-secure-secret
   ```

2. The dashboard must then include a `signature` parameter in the URI.

### Trusted Origins

You can restrict which sources are allowed to invoke the ERST protocol:

```bash
export ERST_TRUSTED_ORIGINS=dashboard,explorer,my-app
```

### Rate Limiting

By default, ERST limits protocol invocations to **5 per minute** to prevent automated abuse.

### Audit Logging

All protocol invocation attempts (both accepted and rejected) are logged for security auditing:

- **Audit Log Path:** `~/.erst/protocol-audit.log`

## Troubleshooting

- **Protocol Not Invoked:** Ensure you have run `protocol:register`. Try unregistering and re-registering: `erst protocol:unregister && erst protocol:register`.
- **Invalid Hash Format:** Ensure the transaction hash is exactly 64 hex characters long.
- **Untrusted Origin:** If you set `ERST_TRUSTED_ORIGINS`, ensure the `source` parameter in the URI matches one of your trusted origins.

## Uninstallation

To remove the protocol handler from your system:

```bash
erst protocol:unregister
```
