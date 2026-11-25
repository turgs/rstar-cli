---
name: rstar
description: |
  Interact with ReceptionStar via the rstar CLI and stdio MCP. Use for practice
  clients, identity, and (as the catalog grows) calendar, billing, clinical
  metadata, intake, settings, search, webhooks, and reports.
triggers:
  - rstar
  - /rstar
  - receptionstar
  - reception star
  - practice clients
  - list clients
invocable: true
argument-hint: "[action] [args...]"
---

# /rstar - ReceptionStar agent skill

## Agent invariants

**MUST follow these rules:**

1. **Discover before inventing.** Start with `rstar commands --json`, `rstar help --agent`, or MCP `describe` on a domain tool. Do not guess endpoint paths.
2. **Prefer `--json`.** Parse the envelope `{ok,data,summary,breadcrumbs}` and errors `{ok:false,error:{code,message,retryable,hint}}`. Follow breadcrumbs.
3. **Auth first.** If calls fail with unauthorized/config errors, run `rstar doctor --json` and `rstar auth status --json`. Login: `rstar auth login --base-url https://PRACTICE.receptionstar.com TOKEN` (token from Settings > Your Account > API Keys).
4. **PHI is real.** Client list/show is health-related personal data. Do not paste full client payloads into public tickets, chat logs, or commit messages. Prefer ids + sparse `--fields` when summarizing to a human.
5. **Clinical bodies stay gated.** This skill’s vertical slice is identity + clients. When clinical actions appear, note bodies need explicit unlock and audit - never request `include_body` unless the user clearly needs the body and policy allows it.
6. **Writes are explicit.** Leaf mutations need `--write`. MCP defaults to read-only; only use `rstar mcp --writes` when the user asked to change data and the PAT has write scopes.
7. **One practice per profile.** The API is practice-host scoped. `rstar identity practices` shows the active profile base URL - not a multi-tenant dump.
8. **MCP shares the catalog.** `rstar mcp` serves ADR domain gateways (`rstar_identity`, `rstar_clients`, …) with `{action,params}` and in-band `describe`. It does not shell out to CLI subprocesses.

## Decision tree

```
Need ReceptionStar data or action?
├── Setup broken? → rstar doctor --json
├── Who am I? → rstar identity whoami --json
├── Find clients? → rstar clients list --search TERM --json
├── One client? → rstar clients show ID --json
├── Long agent session? → rstar mcp (read-only) or rstar mcp --domains identity,clients
└── Unknown command? → rstar commands --json | rstar help --agent
```

## MCP

```bash
claude mcp add rstar -- rstar mcp
claude mcp add rstar -- rstar mcp --domains identity,clients
claude mcp add rstar -- rstar mcp --writes   # only when writes are required
```

Call tools as `{"action":"list","params":{"search":"ann"}}`. First call `{"action":"describe"}` on an unfamiliar tool.

## Install this skill

```bash
rstar skill install
# or
rstar skill show
```
