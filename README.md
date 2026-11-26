# rstar

Command-line tool for [ReceptionStar](https://receptionstar.com).

Use ReceptionStar from your terminal - useful on its own, and for coding agents that run shell commands or speak MCP.

## Status

Early preview.

**Practices:** use hosted MCP in Reception Star (ChatGPT / Claude). You do not need this CLI for normal AI assistants.

**Local MCP (`rstar mcp`) is frozen** for advanced users only (terminal / coding agents on your laptop). No owner-facing roadmap; hosted gets new capabilities first. This repo still owns the shared tool catalog.

This release includes:

- Auth profiles (practice base URL + staff personal access token)
- JSON envelope for agents (`--json`)
- `doctor`, `commands --json`, `help --agent`
- Identity + clients + calendar/billing **reads** against live `/api/v1`
- Agent skill (`rstar skill install`)
- `rstar mcp` stdio server (domain gateways, read-only by default)
- Release binaries, install script, and explicit `rstar upgrade`

## Install

### Install script (recommended)

Downloads the latest GitHub Release for your OS/arch, verifies the SHA-256 against `checksums.txt`, and installs into `~/.local/bin` (or `~/bin` when that is already on your `PATH`):

```bash
curl -fsSL https://raw.githubusercontent.com/turgs/rstar-cli/main/scripts/install.sh | bash
```

Optional environment:

| Variable | Purpose |
| --- | --- |
| `RSTAR_BIN_DIR` | Install directory |
| `RSTAR_VERSION` | Pin a version (no leading `v`) |

### GitHub Release

Prebuilt archives for darwin/linux/windows (amd64 and arm64) are published on every `v*` tag:

https://github.com/turgs/rstar-cli/releases

Verify the archive hash from `checksums.txt` on the same release, extract, and place `rstar` on your `PATH`.

### Build from source

You need [Go](https://go.dev/dl/) 1.25 or newer.

```bash
git clone https://github.com/turgs/rstar-cli.git
cd rstar-cli
make build
# or: go build -trimpath -o rstar ./cmd/rstar
```

Move `rstar` onto your `PATH`, for example:

```bash
mkdir -p ~/.local/bin
mv rstar ~/.local/bin/
```

### Go install

```bash
go install github.com/turgs/rstar-cli/cmd/rstar@latest
```

`rstar upgrade` does not rewrite `go install` binaries; re-run `go install …@latest` instead.

### Homebrew / package managers

Not published yet. Basecamp and Fizzy ship Homebrew casks via `basecamp/tap`; rstar will follow when a tap is ready. Until then use the install script or release archives. `rstar upgrade` detects Homebrew/Scoop/Nix paths and prints guidance instead of mutating those installs.

## Upgrade

There is **no** background or silent auto-update on ordinary commands.

```bash
rstar upgrade
rstar upgrade --json
```

What happens depends on how the CLI was installed:

| Install method | `rstar upgrade` behavior |
| --- | --- |
| Install script / tarball under your home directory | Downloads the latest release archive, verifies SHA-256 via `checksums.txt`, swaps the binary, confirms `rstar version` |
| `go install` | Refuses; hint is `go install github.com/turgs/rstar-cli/cmd/rstar@latest` |
| Homebrew / Scoop / Nix / system paths | Refuses with method-specific guidance (no silent rewrite) |
| Dev / `0.0.0-dev` builds | No-op with install-script hint |

After a successful native upgrade, `rstar version` and `rstar doctor` report the new build (catalog is compiled into the binary).

## Setup

1. In ReceptionStar, open **Settings > Your Account > API Keys** and create a token (`rs_pat_…`).
2. Sign the CLI into your practice host:

```bash
rstar auth login --base-url https://YOUR_PRACTICE.receptionstar.com rs_pat_…
rstar doctor
rstar identity whoami --json
```

Tokens are stored in the OS keyring when available, otherwise a locked-down file under your user config directory.

## Usage

```bash
rstar version
rstar doctor --json
rstar commands --json
rstar help --agent
rstar upgrade

rstar identity whoami --json
rstar clients list --json
rstar clients list --search ann --json
rstar clients show CLIENT_ID --json

rstar skill install
rstar mcp                          # stdio MCP, read-only
rstar mcp --domains identity,clients
rstar mcp --writes                 # opt in to write actions
```

### Agent JSON envelope

```json
{ "ok": true, "data": {}, "summary": "…", "breadcrumbs": [] }
```

Errors:

```json
{ "ok": false, "error": { "code": "unauthorized", "message": "…", "retryable": false, "hint": "…" } }
```

### MCP

Register with an MCP client, for example:

```bash
claude mcp add rstar -- rstar mcp
```

Each domain is one tool (`rstar_identity`, `rstar_clients`, …). Call with `{"action":"…","params":{…}}`. Every tool supports in-band `describe`.

## Configuration

| Source | Purpose |
| --- | --- |
| `rstar auth login` | Profile base URL + token |
| `--profile` / `RSTAR_PROFILE` | Named profile |
| `--base-url` / `RSTAR_BASE_URL` | Override practice origin |
| `--token` / `RSTAR_TOKEN` | Override PAT (prefer keyring) |
| `RSTAR_CONFIG_DIR` | Config directory (tests/advanced) |

## Development

```bash
go test ./...
make smoke
```

Release (maintainers): push a `v*` tag; GitHub Actions runs tests and GoReleaser publishes archives + `checksums.txt`.

```bash
git tag v0.1.0
git push origin v0.1.0
```

## License

[MIT](LICENSE)
