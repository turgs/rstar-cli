---
name: rstar
description: |
  Interact with ReceptionStar via the rstar CLI and stdio MCP. Use for practice
  identity, clients, calendar (appointments), billing (invoices/unbilled), and
  (as the catalog grows) clinical metadata, intake, settings, search, webhooks,
  and reports.
triggers:
  - rstar
  - /rstar
  - receptionstar
  - reception star
  - practice clients
  - list clients
  - appointments
  - invoices
invocable: true
argument-hint: "[action] [args...]"
---

# /rstar - ReceptionStar agent skill

## Agent invariants

**MUST follow these rules:**

1. **Discover before inventing.** Start with `rstar commands --json`, `rstar help --agent`, or MCP `describe` on a domain tool. Do not guess endpoint paths.
2. **Prefer `--json`.** Parse the envelope `{ok,data,summary,breadcrumbs}` and errors `{ok:false,error:{code,message,retryable,hint}}`. Follow breadcrumbs.
3. **Auth first.** If calls fail with unauthorized/config errors, run `rstar doctor --json` and `rstar auth status --json`. Login: `rstar auth login --base-url https://PRACTICE.receptionstar.com TOKEN` (token from Settings > Your Account > API Keys).
4. **PHI is real.** Client list/show is health-related personal data. Do not paste full client payloads into public tickets, chat logs, or commit messages. Prefer ids + sparse `--fields` when summarizing to a human. Invoice show is P2 — prefer `--fields` or `--view summary`.
5. **Clinical bodies stay gated.** Clinical note bodies need explicit unlock and audit - never request `include_body` unless the user clearly needs the body and policy allows it. Clinical domain is not implemented in this slice.
6. **Writes are explicit.** MCP defaults to read-only; only use `rstar mcp --writes` when the user asked to change data and the PAT has write scopes. CLI leaf mutations (when present) need `--write`. Calendar/billing **writes** stay not implemented. Client writes go through MCP + catalog (or CLI with `--write` when wired).
7. **One practice per profile.** The API is practice-host scoped. `rstar identity practices` shows the active profile base URL - not a multi-tenant dump.
8. **MCP shares the catalog.** `rstar mcp` serves ADR domain gateways (`rstar_identity`, `rstar_clients`, `rstar_calendar`, `rstar_billing`, …) with `{action,params}` and in-band `describe`. It does not shell out to CLI subprocesses.

## Decision tree

```
Need ReceptionStar data or action?
├── Setup broken? → rstar doctor --json
├── Who am I? → rstar identity whoami --json
├── Find clients? → rstar clients list --search TERM --json
├── One client? → rstar clients show ID --json
├── Appointments? → rstar calendar list --from-date YYYY-MM-DD --json
├── One appointment? → rstar calendar show ID --json
├── Appointment types? → rstar calendar types --json
├── Invoices? → rstar billing invoices --json
├── One invoice? → rstar billing invoice ID --view summary --json
├── Unbilled? → rstar billing unbilled --json
├── Long agent session? → rstar mcp (read-only) or rstar mcp --domains identity,clients,calendar,billing
└── Unknown command? → rstar commands --json | rstar help --agent
```

## MCP

```bash
claude mcp add rstar -- rstar mcp
claude mcp add rstar -- rstar mcp --domains identity,clients,calendar,billing
claude mcp add rstar -- rstar mcp --writes   # only when writes are required
```

Call tools as `{"action":"list","params":{"search":"ann"}}` (clients) or
`{"action":"list","params":{"from_date":"2026-01-01"}}` (calendar). First call
`{"action":"describe"}` on an unfamiliar tool.

## Install this skill

```bash
rstar skill install
# or
rstar skill show
```
