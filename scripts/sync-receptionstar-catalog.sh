#!/usr/bin/env bash
# Open or update a turgs/ReceptionStar PR with config/mcp/catalog.json from this CLI tip.
# Intended for GitHub Actions (turgs/rstar-issues#508 approach A).
#
# Env:
#   GH_TOKEN   PAT with contents + pull_requests on turgs/ReceptionStar
#   CLI_SHA    full git SHA to record in the PR body (default: git rev-parse HEAD)
#   RS_PATH    path to ReceptionStar checkout (default: ../ReceptionStar)
#   CLI_PATH   path to this rstar-cli checkout (default: .)
set -euo pipefail

CLI_PATH="$(cd "${CLI_PATH:-.}" && pwd)"
RS_PATH="$(cd "${RS_PATH:-../ReceptionStar}" && pwd)"
CLI_SHA="${CLI_SHA:-$(git -C "$CLI_PATH" rev-parse HEAD)}"
SHORT="$(printf '%s' "$CLI_SHA" | cut -c1-12)"
BRANCH="bot/mcp-catalog-sync"
REPO="turgs/ReceptionStar"

if [[ -z "${GH_TOKEN:-}" ]]; then
  echo "GH_TOKEN is required" >&2
  exit 1
fi

if ! command -v gh >/dev/null 2>&1; then
  echo "gh is required" >&2
  exit 1
fi

if ! command -v go >/dev/null 2>&1; then
  echo "go is required" >&2
  exit 1
fi

if ! command -v python3 >/dev/null 2>&1; then
  echo "python3 is required" >&2
  exit 1
fi

echo "Exporting catalog at ${CLI_SHA} from ${CLI_PATH} ..."
(cd "$CLI_PATH" && go run ./cmd/rstar-catalog-export > /tmp/catalog-fresh.json)

python3 - <<'PY'
import json
from pathlib import Path
doc = json.loads(Path("/tmp/catalog-fresh.json").read_text())
if not isinstance(doc.get("domains"), list) or not doc["domains"]:
    raise SystemExit("exporter: empty domains")
if len(doc.get("rstar_cli_commit") or "") < 7:
    raise SystemExit(f"exporter: bad rstar_cli_commit {doc.get('rstar_cli_commit')!r}")
if len(doc.get("domains_sha256") or "") != 64:
    raise SystemExit(f"exporter: bad domains_sha256 {doc.get('domains_sha256')!r}")
print(f"pin commit={doc['rstar_cli_commit']} sha256={doc['domains_sha256']}")
PY

cd "$RS_PATH"
git config user.name "rstar-cli catalog sync"
git config user.email "41898282+github-actions[bot]@users.noreply.github.com"

git fetch origin master
git checkout -B "$BRANCH" origin/master

mkdir -p config/mcp
cp /tmp/catalog-fresh.json config/mcp/catalog.json

if git diff --quiet -- config/mcp/catalog.json; then
  echo "ReceptionStar catalog.json already matches this CLI tip — nothing to do."
  exit 0
fi

git add config/mcp/catalog.json
git commit -m "chore: sync MCP catalog from rstar-cli ${SHORT}

Producer export from turgs/rstar-cli@${CLI_SHA}.
Board: turgs/rstar-issues#508. Gate: bin/sync-mcp-catalog --check."

git push -u origin "HEAD:refs/heads/${BRANCH}" --force

TITLE="chore: sync MCP catalog from rstar-cli ${SHORT}"
BODY="Automatic re-sync of \`config/mcp/catalog.json\` from turgs/rstar-cli@\`${CLI_SHA}\`.

Board: https://github.com/turgs/rstar-issues/issues/508

- Export: \`go run ./cmd/rstar-catalog-export\` on rstar-cli
- Gate: \`bin/sync-mcp-catalog --check\` (turgs/rstar-issues#506)
- Diff should be catalog JSON only

Workflow: turgs/rstar-cli \`.github/workflows/sync-receptionstar-catalog.yml\`"

existing="$(gh pr list --repo "$REPO" --head "$BRANCH" --state open --json number --jq '.[0].number // empty')"
if [[ -n "$existing" ]]; then
  gh pr edit "$existing" --repo "$REPO" --title "$TITLE" --body "$BODY"
  echo "Updated existing PR #${existing}"
  gh pr view "$existing" --repo "$REPO" --json url --jq .url
else
  gh pr create --repo "$REPO" --base master --head "$BRANCH" --title "$TITLE" --body "$BODY"
fi
