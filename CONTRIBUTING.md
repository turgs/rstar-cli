# Contributing

Thanks for helping with the ReceptionStar CLI.

## Setup

1. Install [Go](https://go.dev/dl/) 1.25+.
2. Clone this repository.
3. Run the checks you will see in CI:

```bash
gofmt -l .
go vet ./...
go test -race ./...
go build -o rstar ./cmd/rstar
```

## Pull requests

- Open a PR against `main`.
- Keep the change focused.
- CI must pass before merge (`gofmt`, `go vet`, `go test -race`, build, smoke).

## Domain / ADR invariants

Catalog + MCP rules from the CLI-first plan (ADR 0001 shape) are enforced as **tests**, not a separate linter binary:

- `internal/catalog/adr_invariants_test.go` — unique domains/tools, describe in schema, implemented actions have Method/Path, fail-closed filters, path `{id}` params, destructive ⇒ write
- `internal/mcpserver/adr_invariants_test.go` — read-only default, one tool per domain, **no `os/exec` in production MCP code**
- `internal/apiclient/rails_contract_test.go` / `internal/catalog/rails_contract_test.go` — Rails `/api/v1` response shapes

When you change the catalog or MCP surface, keep those green. Prefer adding another invariant test over a one-off comment.

## MCP catalog snapshot (ReceptionStar hosted MCP)

Source of truth for domain actions is `internal/catalog`. Hosted MCP on private
[turgs/ReceptionStar](https://github.com/turgs/ReceptionStar) loads
`config/mcp/catalog.json`.

After catalog changes merge to `main`, workflow
`.github/workflows/sync-receptionstar-catalog.yml` exports the snapshot and
opens or updates a ReceptionStar PR (board [turgs/rstar-issues#508](https://github.com/turgs/rstar-issues/issues/508)).

Local export (JSON on stdout):

```bash
make catalog-json
# or: go run ./cmd/rstar-catalog-export
```

ReceptionStar still fail-closes on lag via `bin/sync-mcp-catalog --check`
([turgs/rstar-issues#506](https://github.com/turgs/rstar-issues/issues/506)).

Operator setup once: repo secret `RECEPTIONSTAR_CATALOG_SYNC_TOKEN` on
`turgs/rstar-cli` — fine-grained PAT with **Contents** and **Pull requests**
read/write on `turgs/ReceptionStar`.
