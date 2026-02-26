# Update Checker

The erst CLI includes an automatic update checker that notifies users when a newer version is available on GitHub.

## Features

- **Non-intrusive**: Runs in the background without blocking CLI execution
- **Smart caching**: Checks for updates at most once per 24 hours
- **Timeout protection**: Uses 5-second timeout to prevent hanging
- **Silent failures**: Network errors don't interrupt your workflow
- **Easy opt-out**: Simple environment variable to disable

## How It Works

When you run any erst command:

1. **Banner from cache**: A small “Upgrade available” line is shown at the start of the run if the last version check (from a previous run) found a newer release. This is instant and does not hit the network.
2. **Async version check**: A background goroutine pings the version endpoint (GitHub releases API) so the next run can show the banner if an update is available. The check is skipped if update checking is disabled or if a check was already done in the last 24 hours.

So you see the banner once per run when an update is available, without blocking CLI execution.

## Disabling Update Checks

If you prefer not to check for updates, set the environment variable:

```bash
export ERST_NO_UPDATE_CHECK=1
```

Or run commands with:

```bash
ERST_NO_UPDATE_CHECK=1 erst <command>
```

## Cache Location

The update checker stores its cache in:

- **Linux/macOS**: `~/.cache/erst/last_update_check`
- **Windows**: `%LOCALAPPDATA%\erst\last_update_check`

The cache contains:

- Last check timestamp
- Latest known version

## Notification Example

When an update is available, you'll see a one-line banner at the start of the run (to stderr):

```
Upgrade available: v1.2.3 — run 'go install github.com/dotandev/hintents/cmd/erst@latest' to update
```

## Building with Version Information

To set the version during build:

```bash
go build -ldflags "-X main.Version=v1.2.3" -o erst ./cmd/erst
```

Without this flag, the version defaults to "dev" and update checking is skipped.

## Privacy & Security

- Only checks the official GitHub repository
- Uses HTTPS for all API calls
- No personal information is collected or transmitted
- Only notifies - never auto-updates or executes code
- Respects GitHub API rate limits

## Technical Details

- **API Endpoint**: `https://api.github.com/repos/dotandev/hintents/releases/latest`
- **Check Interval**: 24 hours
- **Request Timeout**: 5 seconds
- **Version Comparison**: Uses semantic versioning (via hashicorp/go-version)
