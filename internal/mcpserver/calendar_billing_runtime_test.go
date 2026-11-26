package mcpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/turgs/rstar-cli/internal/apiclient"
	"github.com/turgs/rstar-cli/internal/catalog"
)

// Runtime proof for turgs/rstar-issues#505: stdio MCP → Dispatcher → /api/v1 calendar/billing.

func TestMCPCalendarListDefaultsAndBoolFalse(t *testing.T) {
	var saw []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/engagements" {
			t.Fatalf("path %s", r.URL.Path)
		}
		saw = append(saw, r.URL.Query().Get("is_appointment"))
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{"engagements": []any{map[string]any{"id": "e1", "is_appointment": true}}},
		})
	}))
	defer srv.Close()

	client := apiclient.New(srv.URL, "tok")
	client.HTTPClient = srv.Client()
	s, err := New(client, catalog.ProfileInfo{}, Config{
		ReadOnly: true,
		Domains:  []string{"calendar"},
	})
	if err != nil {
		t.Fatal(err)
	}
	dom := s.Domains()[0]
	if len(dom.Actions) != 3 { // list, show, list_appointment_types (writes dropped)
		t.Fatalf("read-only actions: %+v", dom.Actions)
	}

	// default true
	raw, _ := json.Marshal(map[string]any{"action": "list", "params": map[string]any{}})
	res, err := s.handleTool(context.Background(), nil, dom, &mcp.CallToolRequest{
		Params: &mcp.CallToolParamsRaw{Name: dom.Tool, Arguments: raw},
	})
	if err != nil || res.IsError {
		t.Fatalf("list default: err=%v res=%+v", err, res)
	}

	// JSON bool false must survive (MCP unmarshals booleans as bool)
	raw, _ = json.Marshal(map[string]any{"action": "list", "params": map[string]any{"is_appointment": false}})
	res, err = s.handleTool(context.Background(), nil, dom, &mcp.CallToolRequest{
		Params: &mcp.CallToolParamsRaw{Name: dom.Tool, Arguments: raw},
	})
	if err != nil || res.IsError {
		t.Fatalf("list false: err=%v res=%+v", err, res)
	}

	if len(saw) != 2 || saw[0] != "true" || saw[1] != "false" {
		t.Fatalf("is_appointment queries: %v", saw)
	}
}

func TestMCPBillingUnbilledAndWriteDenied(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/invoices/unbilled_appointments" {
			t.Fatalf("path %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{"synthetic": true, "clients": []any{}},
		})
	}))
	defer srv.Close()
	client := apiclient.New(srv.URL, "tok")
	client.HTTPClient = srv.Client()
	s, err := New(client, catalog.ProfileInfo{}, Config{ReadOnly: true, Domains: []string{"billing"}})
	if err != nil {
		t.Fatal(err)
	}
	dom := s.Domains()[0]

	raw, _ := json.Marshal(map[string]any{"action": "list_unbilled_appointments"})
	res, err := s.handleTool(context.Background(), nil, dom, &mcp.CallToolRequest{
		Params: &mcp.CallToolParamsRaw{Name: dom.Tool, Arguments: raw},
	})
	if err != nil || res.IsError {
		t.Fatalf("%v %+v", err, res)
	}
	text := res.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, `"synthetic"`) && !strings.Contains(text, "clients") {
		// envelope data may nest
		var env map[string]any
		_ = json.Unmarshal([]byte(text), &env)
		data, _ := env["data"].(map[string]any)
		if data["synthetic"] != true {
			t.Fatalf("payload %s", text)
		}
	}

	// write action not even in read-only domain tool enum — calling via dispatcher is denied
	raw, _ = json.Marshal(map[string]any{"action": "invoice_unbilled_appointments", "params": map[string]any{}})
	res, err = s.handleTool(context.Background(), nil, dom, &mcp.CallToolRequest{
		Params: &mcp.CallToolParamsRaw{Name: dom.Tool, Arguments: raw},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsError {
		t.Fatal("expected write denied or unknown")
	}
}

func TestMCPCalendarShowPathInjection(t *testing.T) {
	// Must not hit network with walked path.
	hit := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hit = true
		t.Fatalf("should not reach server: %s", r.URL.Path)
	}))
	defer srv.Close()
	client := apiclient.New(srv.URL, "tok")
	client.HTTPClient = srv.Client()
	s, err := New(client, catalog.ProfileInfo{}, Config{ReadOnly: true, Domains: []string{"calendar"}})
	if err != nil {
		t.Fatal(err)
	}
	dom := s.Domains()[0]
	raw, _ := json.Marshal(map[string]any{"action": "show", "params": map[string]any{"id": "../admin"}})
	res, err := s.handleTool(context.Background(), nil, dom, &mcp.CallToolRequest{
		Params: &mcp.CallToolParamsRaw{Name: dom.Tool, Arguments: raw},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsError {
		t.Fatal("expected error")
	}
	if hit {
		t.Fatal("request escaped to server")
	}
}
