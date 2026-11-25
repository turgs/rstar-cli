package catalog

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/turgs/rstar-cli/internal/apiclient"
	"github.com/turgs/rstar-cli/internal/output"
)

// ProfileInfo is optional practice context for identity.practices.
type ProfileInfo struct {
	ProfileName string
	BaseURL     string
}

// Dispatcher runs catalog actions through the shared API client.
type Dispatcher struct {
	Client  *apiclient.Client
	Profile ProfileInfo
	// AllowWrites must be true to run non-read-only actions.
	AllowWrites bool
}

// Result is a successful dispatch payload.
type Result struct {
	Data    any
	Summary string
	Meta    map[string]any
}

// Call runs domain/action with params.
func (d *Dispatcher) Call(ctx context.Context, domainKey, action string, params map[string]any) (*Result, error) {
	if params == nil {
		params = map[string]any{}
	}
	dom, ok := ByKey(domainKey)
	if !ok {
		return nil, output.NewError(output.CodeDomainDenied, fmt.Sprintf("unknown domain %q", domainKey), output.ExitUsage).
			WithHint("Known domains: " + strings.Join(DomainKeys(), ", "))
	}

	if action == DescribeAction || action == "" {
		target, _ := params["action"].(string)
		payload, err := dom.DescribePayload(target)
		if err != nil {
			return nil, output.NewError(output.CodeUnknownAction, err.Error(), output.ExitUsage)
		}
		return &Result{Data: payload, Summary: "describe " + dom.Key}, nil
	}

	act, ok := FindAction(dom, action)
	if !ok {
		names := make([]string, 0, len(dom.Actions))
		for _, a := range dom.Actions {
			names = append(names, a.Name)
		}
		return nil, output.NewError(output.CodeUnknownAction,
			fmt.Sprintf("unknown action %q on %s", action, dom.Key), output.ExitUsage).
			WithHint("Actions: " + strings.Join(names, ", ") + ", " + DescribeAction)
	}

	if !act.ReadOnly && !d.AllowWrites {
		return nil, output.NewError(output.CodeWriteDenied,
			fmt.Sprintf("action %s.%s is a write; enable writes to continue", dom.Key, act.Name),
			output.ExitForbidden).
			WithHint("CLI: re-run with write intent once confirmed. MCP: start with 'rstar mcp --writes'.")
	}

	if !act.Implemented {
		return nil, output.NewError(output.CodeNotImpl,
			fmt.Sprintf("%s.%s is not implemented in this CLI build yet", dom.Key, act.Name),
			output.ExitAPI).
			WithHint("Vertical slice today: identity + clients. See catalog and ADR 0001.")
	}

	switch dom.Key {
	case "identity":
		return d.dispatchIdentity(ctx, act, params)
	case "clients":
		return d.dispatchClients(ctx, act, params)
	default:
		return nil, output.NewError(output.CodeNotImpl, "domain not wired", output.ExitAPI)
	}
}

func (d *Dispatcher) dispatchIdentity(ctx context.Context, act Action, _ map[string]any) (*Result, error) {
	switch act.Name {
	case "whoami":
		res, err := d.Client.Get(ctx, "/heart/authed", nil)
		if err != nil {
			return nil, err
		}
		data := map[string]any{
			"api":      res.Data(),
			"base_url": d.Profile.BaseURL,
			"profile":  d.Profile.ProfileName,
		}
		if res.Body != nil {
			if rl, ok := res.Body["rate_limit"]; ok {
				data["rate_limit"] = rl
			}
		}
		return &Result{Data: data, Summary: "authenticated"}, nil
	case "practices":
		data := map[string]any{
			"active_profile": d.Profile.ProfileName,
			"base_url":       d.Profile.BaseURL,
			"note":           "The product API is practice-host scoped. One CLI profile maps to one practice base URL.",
		}
		return &Result{Data: data, Summary: "active practice profile"}, nil
	default:
		return nil, output.NewError(output.CodeUnknownAction, act.Name, output.ExitUsage)
	}
}

func (d *Dispatcher) dispatchClients(ctx context.Context, act Action, params map[string]any) (*Result, error) {
	switch act.Name {
	case "list":
		q := url.Values{}
		copyQuery(q, params, "page", "per_page", "search", "fields", "from_date", "to_date")
		res, err := d.Client.Get(ctx, "/clients", q)
		if err != nil {
			return nil, err
		}
		return clientListResult(res, "clients listed"), nil
	case "search":
		q := url.Values{}
		term, _ := stringParam(params, "q")
		if term == "" {
			term, _ = stringParam(params, "search")
		}
		if term == "" {
			return nil, output.ErrUsage("search requires params.q (or search)")
		}
		q.Set("search", term)
		copyQuery(q, params, "page", "per_page", "fields")
		res, err := d.Client.Get(ctx, "/clients", q)
		if err != nil {
			return nil, err
		}
		return clientListResult(res, "clients search"), nil
	case "show":
		id, err := requireID(params)
		if err != nil {
			return nil, err
		}
		q := url.Values{}
		copyQuery(q, params, "fields")
		res, err := d.Client.Get(ctx, "/clients/"+url.PathEscape(id), q)
		if err != nil {
			return nil, err
		}
		return &Result{Data: res.Data(), Summary: "client " + id, Meta: metaFrom(res)}, nil
	case "create":
		body := clientBody(params)
		res, err := d.Client.Post(ctx, "/clients", nil, map[string]any{"client": body})
		if err != nil {
			return nil, err
		}
		return &Result{Data: res.Data(), Summary: "client created", Meta: metaFrom(res)}, nil
	case "update":
		id, err := requireID(params)
		if err != nil {
			return nil, err
		}
		body := clientBody(params)
		delete(body, "id")
		res, err := d.Client.Patch(ctx, "/clients/"+url.PathEscape(id), nil, map[string]any{"client": body})
		if err != nil {
			return nil, err
		}
		return &Result{Data: res.Data(), Summary: "client updated", Meta: metaFrom(res)}, nil
	case "archive":
		id, err := requireID(params)
		if err != nil {
			return nil, err
		}
		res, err := d.Client.Patch(ctx, "/clients/"+url.PathEscape(id)+"/archive", nil, nil)
		if err != nil {
			return nil, err
		}
		return &Result{Data: res.Data(), Summary: "client archived", Meta: metaFrom(res)}, nil
	case "unarchive":
		id, err := requireID(params)
		if err != nil {
			return nil, err
		}
		res, err := d.Client.Patch(ctx, "/clients/"+url.PathEscape(id)+"/unarchive", nil, nil)
		if err != nil {
			return nil, err
		}
		return &Result{Data: res.Data(), Summary: "client unarchived", Meta: metaFrom(res)}, nil
	case "merge":
		id, err := requireID(params)
		if err != nil {
			return nil, err
		}
		loser, _ := stringParam(params, "loser_id")
		if loser == "" {
			return nil, output.ErrUsage("merge requires params.loser_id")
		}
		// Rails ClientsController#merge reads params[:loser_id] (and optional merge_reason).
		body := map[string]any{"loser_id": loser}
		if reason, ok := stringParam(params, "merge_reason"); ok && reason != "" {
			body["merge_reason"] = reason
		}
		res, err := d.Client.Post(ctx, "/clients/"+url.PathEscape(id)+"/merge", nil, body)
		if err != nil {
			return nil, err
		}
		// Merge success is top-level JSON (no data wrapper): {merged, winner_id, ...}.
		// Prefer body["data"] when present; otherwise copy body minus rate_limit/pagination.
		var data any
		if res.Data() != nil {
			data = res.Data()
		} else if res.Body != nil {
			cp := map[string]any{}
			for k, v := range res.Body {
				if k == "rate_limit" || k == "pagination" {
					continue
				}
				cp[k] = v
			}
			data = cp
		}
		return &Result{Data: data, Summary: "clients merged", Meta: metaFrom(res)}, nil
	case "destroy":
		// Rails: DELETE /clients/:id — only succeeds when already archived (422 otherwise).
		id, err := requireID(params)
		if err != nil {
			return nil, err
		}
		res, err := d.Client.Delete(ctx, "/clients/"+url.PathEscape(id), nil)
		if err != nil {
			return nil, err
		}
		// 204 No Content — empty body.
		return &Result{Data: map[string]any{"deleted": true, "id": id}, Summary: "client deleted", Meta: metaFrom(res)}, nil
	default:
		return nil, output.NewError(output.CodeUnknownAction, act.Name, output.ExitUsage)
	}
}

// clientListResult maps the Rails index.json.jb shape into the agent envelope:
//
//	API:  { data: { clients: [...] }, pagination: {...}, rate_limit: {...} }
//	CLI:  envelope.data = { clients: [...] }; envelope.meta = pagination + rate_limit
//
// Avoids the old double-wrap envelope.data.data.clients.
func clientListResult(res *apiclient.Response, summary string) *Result {
	return &Result{Data: res.Data(), Summary: summary, Meta: metaFrom(res)}
}

func metaFrom(res *apiclient.Response) map[string]any {
	if res == nil || res.Body == nil {
		return nil
	}
	m := map[string]any{}
	if p, ok := res.Body["pagination"]; ok {
		m["pagination"] = p
	}
	if rl, ok := res.Body["rate_limit"]; ok {
		m["rate_limit"] = rl
	}
	if len(m) == 0 {
		return nil
	}
	return m
}

func requireID(params map[string]any) (string, error) {
	id, _ := stringParam(params, "id")
	if id == "" {
		return "", output.ErrUsage("params.id is required")
	}
	return id, nil
}

func stringParam(params map[string]any, key string) (string, bool) {
	v, ok := params[key]
	if !ok || v == nil {
		return "", false
	}
	switch t := v.(type) {
	case string:
		return t, true
	case fmt.Stringer:
		return t.String(), true
	default:
		return fmt.Sprint(t), true
	}
}

func copyQuery(q url.Values, params map[string]any, keys ...string) {
	for _, k := range keys {
		if s, ok := stringParam(params, k); ok && s != "" {
			q.Set(k, s)
		}
	}
}

func clientBody(params map[string]any) map[string]any {
	body := map[string]any{}
	// Prefer nested client object when provided.
	if nested, ok := params["client"].(map[string]any); ok {
		for k, v := range nested {
			body[k] = v
		}
	}
	for _, k := range []string{
		"first_name", "last_name", "email", "mobile_phone", "other_phone",
		"date_of_birth", "gender", "title", "street_address", "suburb", "state",
		"postcode", "country", "extra_information",
	} {
		if v, ok := params[k]; ok {
			body[k] = v
		}
	}
	return body
}
