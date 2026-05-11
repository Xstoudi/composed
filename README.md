# composed

`composed` is a small Go CLI that keeps Docker Compose stacks in sync with a Git repository.

It fetches `origin`, computes what changed under your stacks folder, and applies only the required stack actions (`down` and/or `up`). It is designed for unattended deployment-style runs (cron, timers, CI runners, etc.).

## Features

- Git-aware stack reconciliation (changed stacks only)
- File lock with stale-process recovery to prevent overlapping runs
- Stack discovery from a configurable stacks folder
- Safe mode options:
  - `--dry-run` (plan only, no Docker actions)
  - `--force` (force all discovered stacks to `up`)
- Structured logging with optional JSON output (`--json`)
- Optional notifications via Shoutrrr for created, updated, deleted, and error events

## How It Works

At runtime, `composed`:

1. Resolves the working directory (optional positional argument).
2. Loads config from `composed.toml` / `composed.yaml` / `composed.yml` (if present).
3. Acquires a lock file.
4. Fetches from `origin` and fast-forwards local branch (clean worktree required).
5. Computes changed files under the stacks folder.
6. Maps changed files to stacks and decides actions (`down`, `up`, or no-op).
7. Runs Docker Compose actions for the selected stacks.
8. Logs a structured execution summary.

## Requirements

- Go `1.26+` (module targets `go 1.26`)
- Docker Engine with Docker Compose available
- A Git repository with an `origin` remote

## Installation

### One-line install (Linux & macOS)

```bash
curl -sSfL https://raw.githubusercontent.com/Xstoudi/composed/main/install.sh | sh
```

This downloads the latest release binary for your OS/arch and installs it to `/usr/local/bin`.

Override the install directory:

```bash
INSTALL_DIR=~/.local/bin curl -sSfL https://raw.githubusercontent.com/Xstoudi/composed/main/install.sh | sh
```

Pin a specific version:

```bash
VERSION=v1.2.3 curl -sSfL https://raw.githubusercontent.com/Xstoudi/composed/main/install.sh | sh
```

### Build locally

```bash
go build -o composed ./cmd
```

### Install with Go

```bash
go install ./cmd
```

## Quick Start

Create a config file in your repo root (example `composed.toml`):

```toml
stacksFolder = "stacks"
lockFile = "/tmp/composed.lock"
notifyURL = "ntfy://token@ntfy.example.com/infrastructure"
```

Run in current directory:

```bash
./composed
```

Run against another directory:

```bash
./composed /path/to/repo
```

## CLI Usage

```text
composed [flags] [working-directory]
```

### Flags

- `--dry-run` : compute and log actions without running Docker Compose
- `--force` : force all discovered stacks to be started (`up`)
- `--verbose` : enable debug logs
- `--json` : emit logs in JSON format

Examples:

```bash
./composed --dry-run
./composed --force --verbose
./composed --json --dry-run /path/to/repo
```

## Configuration

`composed` reads the first existing file among:

1. `composed.toml`
2. `composed.yaml`
3. `composed.yml`

Supported keys:

- `stacksFolder` (default: `stacks`)
- `lockFile` (default: `/tmp/.composed-lock`)
- `notifyURL` (default: empty / disabled)

## Notifications

When `notifyURL` is configured, `composed` sends one aggregated notification per non-dry run for meaningful events only:

- created stacks
- updated stacks
- deleted stacks
- run errors

No notification is sent for pure no-op runs.

## Logging

Default logs are structured text via `log/slog`.

Use `--json` for machine-readable logs, for example when shipping to ELK/Loki/Cloud logging pipelines.

## Exit Behavior

- Returns non-zero on errors (config, lock, git update, stack planning, compose execution).
- Exits early with no compose actions when:
  - no relevant repo changes are detected, or
  - `--dry-run` is enabled.

## Development

Run tests:

```bash
go test ./...
```

Format code:

```bash
gofmt -w ./cmd ./internal
```

## Current Limitations

- Requires a clean worktree before pull/fetch reconciliation.
- Assumes deployment branch tracks `origin/<current-branch>`.
- Focused on compose stacks under one configured root folder.

## License

This project is licensed under the **GNU Affero General Public License v3.0** — see the [LICENSE](LICENSE) file for details.

