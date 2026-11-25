# rstar-cli

ReceptionStar command-line tool. Binary name: **`rstar`**.

This is the agent spine for coding agents (layout: product API → CLI + skill → stdio MCP). Hosted MCP is a later thin face of the same catalog.

| | |
| --- | --- |
| Product app | [`turgs/ReceptionStar`](https://github.com/turgs/ReceptionStar) |
| Board | [`turgs/rstar-issues`](https://github.com/turgs/rstar-issues) |
| Bootstrap ticket | [rstar-issues#502](https://github.com/turgs/rstar-issues/issues/502) |
| Commands + MCP | [rstar-issues#479](https://github.com/turgs/rstar-issues/issues/479) |

## Status

Bootstrap only. Ships `rstar version` and a `rstar doctor` stub. Auth, leaf API commands, agent skill, and `rstar mcp` land on **#479**.

## Requirements

- Go 1.22+

## Build

```bash
go build -o rstar ./cmd/rstar
./rstar version
./rstar doctor
```

## Install

TBD (release binaries, install script, Homebrew later). For now, build from source.

## Test

```bash
go test ./...
```

## Layout

```text
cmd/rstar/          # binary entrypoint
internal/version/   # version string (link-time override ready)
```

HTTP client and domain catalog will live in this module until hosted MCP needs a shared extract (no separate SDK repo in v1).

## License

MIT
