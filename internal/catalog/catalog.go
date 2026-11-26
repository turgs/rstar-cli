// Package catalog is the single source of domain gateway actions for CLI and MCP.
//
// Hosted MCP (board turgs/rstar-issues#279) must reuse this module (copy/sync or
// shared extract later) — not a second hand-maintained tool matrix in Rails.
//
// Shape matches ADR 0001 (docs-internal/adr/0001-mcp-domain-gateway-catalog.md
// in ReceptionStar): one tool per domain, call {"action","params"}, in-band
// describe, read-only default.
package catalog

import (
	"fmt"
	"sort"
	"strings"
)

// DescribeAction is the in-band discovery action on every domain tool.
const DescribeAction = "describe"

// Param describes one action parameter.
type Param struct {
	Name        string `json:"name"`
	In          string `json:"in"` // path | query | body
	Type        string `json:"type,omitempty"`
	Required    bool   `json:"required,omitempty"`
	Description string `json:"description,omitempty"`
}

// Action is one curated gateway operation.
type Action struct {
	Name        string `json:"action"`
	Summary     string `json:"summary"`
	ReadOnly    bool   `json:"readonly"`
	Destructive bool   `json:"destructive,omitempty"`
	Synthetic   bool   `json:"synthetic,omitempty"`
	Implemented bool   `json:"implemented"`
	// Method + Path are relative to /api/v1 when Implemented (path tokens {id}).
	Method string  `json:"method,omitempty"`
	Path   string  `json:"path,omitempty"`
	Params []Param `json:"params,omitempty"`
	// BodyKey wraps JSON body under this key when set (Rails convention).
	BodyKey string `json:"body_key,omitempty"`
}

// Domain is one MCP tool / CLI resource group.
type Domain struct {
	Key     string   `json:"key"`  // short: identity, clients
	Tool    string   `json:"tool"` // MCP: rstar_identity
	Title   string   `json:"title"`
	Blurb   string   `json:"blurb"`
	Actions []Action `json:"actions"`
}

// All returns the v1 catalog. Identity, clients, and calendar/billing reads are
// implemented; remaining ADR domains stay discoverable via describe.
func All() []Domain {
	return []Domain{
		identityDomain(),
		clientsDomain(),
		calendarDomain(),
		billingDomain(),
		// Roadmap domains (ADR 0001) — describe works; actions not implemented yet.
		roadmapDomain("clinical", "rstar_clinical", "ReceptionStar Clinical",
			"Case note and client file metadata. Bodies gated (PHI).",
			[]Action{
				{Name: "list_case_notes", Summary: "List case note metadata", ReadOnly: true},
				{Name: "show_case_note", Summary: "Show case note metadata", ReadOnly: true},
			}),
		roadmapDomain("intake", "rstar_intake", "ReceptionStar Intake",
			"Intake templates, requests, and submissions.",
			[]Action{
				{Name: "list_templates", Summary: "List intake templates", ReadOnly: true},
				{Name: "list_requests", Summary: "List intake requests", ReadOnly: true},
			}),
		roadmapDomain("settings", "rstar_settings", "ReceptionStar Settings",
			"Locations, practitioners, closures, holidays, hours.",
			[]Action{
				{Name: "list_locations", Summary: "List locations", ReadOnly: true},
				{Name: "list_practitioners", Summary: "List practitioners", ReadOnly: true},
			}),
		roadmapDomain("search", "rstar_search", "ReceptionStar Search",
			"Cross-resource search.",
			[]Action{
				{Name: "query", Summary: "Run a search query", ReadOnly: true},
			}),
		roadmapDomain("webhooks", "rstar_webhooks", "ReceptionStar Webhooks",
			"Webhook endpoints and deliveries.",
			[]Action{
				{Name: "list", Summary: "List webhooks", ReadOnly: true},
				{Name: "show", Summary: "Show a webhook", ReadOnly: true},
			}),
		roadmapDomain("reports", "rstar_reports", "ReceptionStar Reports",
			"Bounded report runs.",
			[]Action{
				{Name: "list_available", Summary: "List available reports", ReadOnly: true},
				{Name: "run", Summary: "Run a bounded report", ReadOnly: true},
			}),
	}
}

func identityDomain() Domain {
	return Domain{
		Key:   "identity",
		Tool:  "rstar_identity",
		Title: "ReceptionStar Identity",
		Blurb: "Who you are and which practice this profile targets.",
		Actions: []Action{
			{
				Name: "whoami", Summary: "Return the authenticated user and practice host",
				ReadOnly: true, Implemented: true, Method: "GET", Path: "/heart/authed",
			},
			{
				Name: "practices", Summary: "Show the active CLI profile practice (API is practice-scoped)",
				ReadOnly: true, Implemented: true,
			},
		},
	}
}

func calendarDomain() Domain {
	return Domain{
		Key:   "calendar",
		Tool:  "rstar_calendar",
		Title: "ReceptionStar Calendar",
		Blurb: "Appointments (API: engagements) and appointment types.",
		Actions: []Action{
			{
				Name: "list", Summary: "List appointments (engagements; defaults is_appointment=true)",
				ReadOnly: true, Implemented: true, Method: "GET", Path: "/engagements",
				Params: []Param{
					{Name: "page", In: "query", Type: "integer"},
					{Name: "per_page", In: "query", Type: "integer", Description: "Default 25, max 100"},
					{Name: "from_date", In: "query", Type: "string", Description: "YYYY-MM-DD (alias: from)"},
					{Name: "to_date", In: "query", Type: "string", Description: "YYYY-MM-DD (alias: to)"},
					{Name: "is_appointment", In: "query", Type: "string", Description: "true|false; CLI defaults true"},
					{Name: "client_id", In: "query", Type: "string"},
					{Name: "client_ids", In: "query", Type: "string", Description: "Comma-separated client ids"},
					{Name: "user_id", In: "query", Type: "string", Description: "Practitioner id"},
					{Name: "user_ids", In: "query", Type: "string"},
					{Name: "location_id", In: "query", Type: "string"},
					{Name: "location_ids", In: "query", Type: "string"},
					{Name: "sort", In: "query", Type: "string", Description: "e.g. start_date:asc"},
				},
			},
			{
				Name: "show", Summary: "Show one appointment (engagement) by id",
				ReadOnly: true, Implemented: true, Method: "GET", Path: "/engagements/{id}",
				Params: []Param{
					{Name: "id", In: "path", Type: "string", Required: true},
				},
			},
			{
				Name: "list_appointment_types", Summary: "List appointment types (not archived)",
				ReadOnly: true, Implemented: true, Method: "GET", Path: "/appointment_types",
				Params: []Param{
					{Name: "page", In: "query", Type: "integer"},
					{Name: "per_page", In: "query", Type: "integer", Description: "Default 25, max 100"},
				},
			},
			// Writes stay roadmap until a later catalog slice.
			{Name: "create", Summary: "Create an appointment", ReadOnly: false, Implemented: false},
			{Name: "update", Summary: "Update an appointment", ReadOnly: false, Implemented: false},
			{Name: "cancel", Summary: "Cancel an appointment", ReadOnly: false, Destructive: true, Implemented: false},
			{Name: "uncancel", Summary: "Uncancel an appointment", ReadOnly: false, Implemented: false},
		},
	}
}

func billingDomain() Domain {
	return Domain{
		Key:   "billing",
		Tool:  "rstar_billing",
		Title: "ReceptionStar Billing",
		Blurb: "Invoices and unbilled appointment composites (P2; prefer sparse fields).",
		Actions: []Action{
			{
				Name: "list_invoices", Summary: "List invoices (paginated)",
				ReadOnly: true, Implemented: true, Method: "GET", Path: "/invoices",
				Params: []Param{
					{Name: "page", In: "query", Type: "integer"},
					{Name: "per_page", In: "query", Type: "integer", Description: "Default 25, max 100"},
					{Name: "search", In: "query", Type: "string"},
					{Name: "status", In: "query", Type: "string", Description: "Invoice status id"},
					{Name: "client_id", In: "query", Type: "string"},
					{Name: "fields", In: "query", Type: "string", Description: "Sparse field list"},
				},
			},
			{
				Name: "show_invoice", Summary: "Show one invoice by id",
				ReadOnly: true, Implemented: true, Method: "GET", Path: "/invoices/{id}",
				Params: []Param{
					{Name: "id", In: "path", Type: "string", Required: true},
					{Name: "fields", In: "query", Type: "string"},
					{Name: "view", In: "query", Type: "string", Description: "summary omits line items/payments"},
				},
			},
			{
				// Same eligibility set as Suggested Invoices (web) / InvoiceBatchCreationService preview.
				Name: "list_unbilled_appointments", Summary: "List clients with eligible unbilled past appointments",
				ReadOnly: true, Implemented: true, Synthetic: true,
				Method: "GET", Path: "/invoices/unbilled_appointments",
				Params: []Param{
					{Name: "page", In: "query", Type: "integer"},
					{Name: "per_page", In: "query", Type: "integer", Description: "Default 25, max 100"},
				},
			},
			{
				Name: "invoice_unbilled_appointments", Summary: "Invoice unbilled appointments",
				ReadOnly: false, Implemented: false, Synthetic: true,
			},
		},
	}
}

func clientsDomain() Domain {
	return Domain{
		Key:   "clients",
		Tool:  "rstar_clients",
		Title: "ReceptionStar Clients",
		Blurb: "Client records (P1 PHI: treat carefully; never echo into public logs).",
		Actions: []Action{
			{
				Name: "list", Summary: "List clients (paginated; excludes archived)",
				ReadOnly: true, Implemented: true, Method: "GET", Path: "/clients",
				Params: []Param{
					{Name: "page", In: "query", Type: "integer"},
					{Name: "per_page", In: "query", Type: "integer", Description: "Default 25, max 100"},
					{Name: "search", In: "query", Type: "string", Description: "Name/email search"},
					{Name: "fields", In: "query", Type: "string", Description: "Sparse field list (comma-separated)"},
					{Name: "from_date", In: "query", Type: "string", Description: "created_at >= YYYY-MM-DD"},
					{Name: "to_date", In: "query", Type: "string", Description: "created_at <= YYYY-MM-DD"},
				},
			},
			{
				Name: "search", Summary: "Search clients by name/email",
				ReadOnly: true, Implemented: true, Method: "GET", Path: "/clients",
				Params: []Param{
					{Name: "q", In: "query", Type: "string", Required: true, Description: "Search term (mapped to search=)"},
					{Name: "page", In: "query", Type: "integer"},
					{Name: "per_page", In: "query", Type: "integer"},
				},
			},
			{
				Name: "show", Summary: "Show one client by id",
				ReadOnly: true, Implemented: true, Method: "GET", Path: "/clients/{id}",
				Params: []Param{
					{Name: "id", In: "path", Type: "string", Required: true},
					{Name: "fields", In: "query", Type: "string"},
				},
			},
			{
				Name: "create", Summary: "Create a client",
				ReadOnly: false, Implemented: true, Method: "POST", Path: "/clients",
				BodyKey: "client",
				Params: []Param{
					{Name: "first_name", In: "body", Type: "string", Required: true},
					{Name: "last_name", In: "body", Type: "string", Required: true},
					{Name: "email", In: "body", Type: "string"},
					{Name: "mobile_phone", In: "body", Type: "string"},
				},
			},
			{
				Name: "update", Summary: "Update a client",
				ReadOnly: false, Implemented: true, Method: "PATCH", Path: "/clients/{id}",
				BodyKey: "client",
				Params: []Param{
					{Name: "id", In: "path", Type: "string", Required: true},
					{Name: "first_name", In: "body", Type: "string"},
					{Name: "last_name", In: "body", Type: "string"},
					{Name: "email", In: "body", Type: "string"},
					{Name: "mobile_phone", In: "body", Type: "string"},
				},
			},
			{
				Name: "archive", Summary: "Archive a client",
				ReadOnly: false, Implemented: true, Method: "PATCH", Path: "/clients/{id}/archive",
				Params: []Param{{Name: "id", In: "path", Type: "string", Required: true}},
			},
			{
				Name: "unarchive", Summary: "Unarchive a client",
				ReadOnly: false, Implemented: true, Method: "PATCH", Path: "/clients/{id}/unarchive",
				Params: []Param{{Name: "id", In: "path", Type: "string", Required: true}},
			},
			{
				Name: "merge", Summary: "Merge loser client into winner (settings.manage)",
				ReadOnly: false, Implemented: true, Method: "POST", Path: "/clients/{id}/merge", Destructive: true,
				Params: []Param{
					{Name: "id", In: "path", Type: "string", Required: true, Description: "Winner client id"},
					{Name: "loser_id", In: "body", Type: "string", Required: true},
					{Name: "merge_reason", In: "body", Type: "string"},
				},
			},
			{
				// Rails destroy refuses unless archived first (422).
				Name: "destroy", Summary: "Permanently delete an archived client",
				ReadOnly: false, Implemented: true, Method: "DELETE", Path: "/clients/{id}", Destructive: true,
				Params: []Param{{Name: "id", In: "path", Type: "string", Required: true}},
			},
		},
	}
}

func roadmapDomain(key, tool, title, blurb string, actions []Action) Domain {
	for i := range actions {
		actions[i].Implemented = false
	}
	return Domain{Key: key, Tool: tool, Title: title, Blurb: blurb, Actions: actions}
}

// ByKey returns a domain by short key or tool name.
func ByKey(key string) (Domain, bool) {
	key = normalizeDomainKey(key)
	for _, d := range All() {
		if d.Key == key || d.Tool == key || strings.TrimPrefix(d.Tool, "rstar_") == key {
			return d, true
		}
	}
	return Domain{}, false
}

// Filter returns domains matching allowlist (empty = all). Unknown keys error.
func Filter(keys []string) ([]Domain, error) {
	if len(keys) == 0 {
		return All(), nil
	}
	var out []Domain
	seen := map[string]bool{}
	for _, k := range keys {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		d, ok := ByKey(k)
		if !ok {
			return nil, fmt.Errorf("unknown domain %q (known: %s)", k, strings.Join(DomainKeys(), ", "))
		}
		if seen[d.Key] {
			continue
		}
		seen[d.Key] = true
		out = append(out, d)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no domains selected")
	}
	return out, nil
}

// FilterReadOnly drops write actions; domains with no remaining actions are omitted.
func FilterReadOnly(domains []Domain) []Domain {
	var out []Domain
	for _, d := range domains {
		var acts []Action
		for _, a := range d.Actions {
			if a.ReadOnly {
				acts = append(acts, a)
			}
		}
		if len(acts) == 0 {
			continue
		}
		d.Actions = acts
		out = append(out, d)
	}
	return out
}

// FindAction returns an action on a domain.
func FindAction(d Domain, name string) (Action, bool) {
	for _, a := range d.Actions {
		if a.Name == name {
			return a, true
		}
	}
	return Action{}, false
}

// DomainKeys returns sorted short keys.
func DomainKeys() []string {
	all := All()
	keys := make([]string, 0, len(all))
	for _, d := range all {
		keys = append(keys, d.Key)
	}
	sort.Strings(keys)
	return keys
}

// Description builds the MCP tool description for a domain.
func (d Domain) Description() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n\n", d.Blurb)
	b.WriteString("Gateway tool: call with {\"action\": \"...\", \"params\": {...}}.\n")
	fmt.Fprintf(&b, "Call {\"action\": %q, \"params\": {\"action\": \"NAME\"}} for an action's full parameter schema.\n\n", DescribeAction)
	b.WriteString("ACTIONS (RO = read-only):\n")
	for _, a := range d.Actions {
		var notes []string
		if a.ReadOnly {
			notes = append(notes, "RO")
		}
		if a.Destructive {
			notes = append(notes, "destructive")
		}
		if a.Synthetic {
			notes = append(notes, "synthetic")
		}
		if !a.Implemented {
			notes = append(notes, "not_implemented")
		}
		suffix := ""
		if len(notes) > 0 {
			suffix = " (" + strings.Join(notes, ", ") + ")"
		}
		fmt.Fprintf(&b, "- %s%s: %s\n", a.Name, suffix, a.Summary)
	}
	return b.String()
}

// InputSchema is the JSON Schema for gateway tool arguments.
func (d Domain) InputSchema() map[string]any {
	actions := make([]any, 0, len(d.Actions)+1)
	for _, a := range d.Actions {
		actions = append(actions, a.Name)
	}
	actions = append(actions, DescribeAction)
	return map[string]any{
		"type":                 "object",
		"required":             []any{"action"},
		"additionalProperties": false,
		"properties": map[string]any{
			"action": map[string]any{
				"type": "string",
				"enum": actions,
			},
			"params": map[string]any{
				"type":        "object",
				"description": "Parameters for the action. Call describe for the action's schema.",
			},
		},
	}
}

// DescribePayload returns in-band describe data.
func (d Domain) DescribePayload(action string) (any, error) {
	if action == "" {
		summaries := make([]map[string]any, 0, len(d.Actions))
		for _, a := range d.Actions {
			summaries = append(summaries, map[string]any{
				"action":      a.Name,
				"summary":     a.Summary,
				"readonly":    a.ReadOnly,
				"destructive": a.Destructive,
				"implemented": a.Implemented,
			})
		}
		return map[string]any{"domain": d.Key, "tool": d.Tool, "actions": summaries}, nil
	}
	a, ok := FindAction(d, action)
	if !ok {
		return nil, fmt.Errorf("unknown action %q in domain %q", action, d.Key)
	}
	return a, nil
}

func normalizeDomainKey(key string) string {
	key = strings.TrimSpace(strings.ToLower(key))
	key = strings.TrimPrefix(key, "rstar_")
	return key
}
