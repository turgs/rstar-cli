# rstar

Command-line tool for [ReceptionStar](https://receptionstar.com).

Use ReceptionStar from your terminal - useful on its own, and for coding agents that run shell commands or speak MCP.

## Status

Early preview. This release includes:

- Auth profiles (practice base URL + staff personal access token)
- JSON envelope for agents (`--json`)
- `doctor`, `commands --json`, `help --agent`
- Identity + clients vertical slice against live `/api/v1`
- Agent skill (`rstar skill install`)
- `rstar mcp` stdio server (domain gateways, read-only by default)

## Install

### Build from source

You need [Go](https://go.dev/dl/) 1.25 or newer.

```bash
git clone https://github.com/turgs/rstar-cli.git
cd rstar-cli
go build -o rstar ./cmd/rstar
```

Move `rstar` onto your `PATH`, for example:

```bash
mkdir -p ~/.local/bin
mv rstar ~/.local/bin/
```

Release binaries, an install script, and package-manager installs will come later.

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

## License

[MIT](LICENSE)
