# Update Checker

The erst CLI includes an automatic update checker that notifies users when a newer version is available on GitHub.

## Features

- **Non-intrusive**: Runs in the background without blocking CLI execution
- **Smart caching**: Checks for updates at most once per 24 hours
- **Timeout protection**: Uses 5-second timeout to prevent hanging
- **Silent failures**: Network errors don't interrupt your workflow
- **Easy opt-out**: Simple environment variable to disable

## How It Works

When you run any erst command, the update checker:

1. Checks if update checking is disabled (via environment variable)
2. Verifies if 24 hours have passed since the last check (using cache)
3. Queries the GitHub API for the latest release
4. Compares the latest version with your current version
5. Displays a friendly notification if an update is available

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

When an update is available, you'll see:

```
💡 A new version (v1.2.3) is available! Run 'go install github.com/dotandev/hintents/cmd/erst@latest' to update.
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
