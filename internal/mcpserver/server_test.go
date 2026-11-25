package mcpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/turgs/rstar-cli/internal/apiclient"
	"github.com/turgs/rstar-cli/internal/catalog"
)

func TestNewUnknownDomainFailsClosed(t *testing.T) {
	c := apiclient.New("http://example", "t")
	_, err := New(c, catalog.ProfileInfo{}, Config{ReadOnly: true, Domains: []string{"nope"}})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNewReadOnlyDropsWrites(t *testing.T) {
	c := apiclient.New("http://example", "t")
	s, err := New(c, catalog.ProfileInfo{}, Config{ReadOnly: true, Domains: []string{"clients"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Domains()) != 1 {
		t.Fatal(s.Domains())
	}
	for _, a := range s.Domains()[0].Actions {
		if !a.ReadOnly {
			t.Fatalf("write action present: %s", a.Name)
		}
	}
}

func TestHandleDescribe(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{}})
	}))
	defer srv.Close()
	client := apiclient.New(srv.URL, "t")
	client.HTTPClient = srv.Client()
	s, err := New(client, catalog.ProfileInfo{ProfileName: "p", BaseURL: srv.URL}, Config{
		ReadOnly: true,
		Domains:  []string{"identity"},
	})
	if err != nil {
		t.Fatal(err)
	}
	dom := s.Domains()[0]
	raw, _ := json.Marshal(map[string]any{"action": "describe"})
	res, err := s.handleTool(context.Background(), nil, dom, &mcp.CallToolRequest{
		Params: &mcp.CallToolParamsRaw{
			Name:      dom.Tool,
			Arguments: raw,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("%+v", res)
	}
	text := res.Content[0].(*mcp.TextContent).Text
	var env map[string]any
	if err := json.Unmarshal([]byte(text), &env); err != nil {
		t.Fatal(err)
	}
	if env["ok"] != true {
		t.Fatalf("%v", env)
	}
}
