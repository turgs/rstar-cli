package catalog

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/turgs/rstar-cli/internal/apiclient"
	"github.com/turgs/rstar-cli/internal/output"
)

// Behaviour tests against Rails-shaped HTTP. Sources:
//   ReceptionStar app/controllers/api/v1/clients_controller.rb
//   ReceptionStar app/views/api/v1/clients/*.json.jb
//   ReceptionStar test/integration/api/v1/clients_controller_test.rb
//   ReceptionStar config/routes/tenanted_api_routes.rb (member archive/unarchive/merge)

func TestDispatchListMatchesRailsIndex(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/clients" {
			t.Fatalf("%s %s", r.Method, r.URL.Path)
		}
		if r.URL.Query().Get("per_page") != "1" {
			t.Fatalf("query %v", r.URL.Query())
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"clients": []any{map[string]any{"id": 7, "full_name": "A B"}},
			},
			"pagination": map[string]any{"page": 1, "per_page": 1, "total_count": 3, "total_pages": 3},
			"rate_limit": map[string]any{"limit": 100, "remaining": 99, "reset": 10},
		})
	}))
	defer srv.Close()

	d := &Dispatcher{Client: apiclient.New(srv.URL, "tok")}
	d.Client.HTTPClient = srv.Client()

	res, err := d.Call(context.Background(), "clients", "list", map[string]any{"per_page": "1"})
	if err != nil {
		t.Fatal(err)
	}
	// envelope.data should be {clients: [...]} — not nested data.data.clients
	data, ok := res.Data.(map[string]any)
	if !ok {
		t.Fatalf("%T", res.Data)
	}
	if _, ok := data["clients"]; !ok {
		t.Fatalf("want clients key, got %#v", data)
	}
	if nested, ok := data["data"]; ok {
		t.Fatalf("double-wrapped data: %#v", nested)
	}
	if res.Meta["pagination"] == nil {
		t.Fatalf("meta pagination missing: %#v", res.Meta)
	}
}

func TestDispatchShowMatchesRailsShow(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/clients/42" {
			t.Fatalf("path %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"client": map[string]any{
					"id": 42, "first_name": "Ann", "last_name": "Lee",
					"medicare_number": nil, "created_at": "2024-01-02T03:04:05Z",
				},
			},
			"rate_limit": map[string]any{"limit": 100, "remaining": 98, "reset": 10},
		})
	}))
	defer srv.Close()

	d := &Dispatcher{Client: apiclient.New(srv.URL, "tok")}
	d.Client.HTTPClient = srv.Client()
	res, err := d.Call(context.Background(), "clients", "show", map[string]any{"id": "42"})
	if err != nil {
		t.Fatal(err)
	}
	data := res.Data.(map[string]any)
	client := data["client"].(map[string]any)
	if client["first_name"] != "Ann" {
		t.Fatalf("%#v", client)
	}
}

func TestDispatchCreateSendsClientWrapperAndAccepts201(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/clients" {
			t.Fatalf("%s %s", r.Method, r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		var m map[string]any
		if err := json.Unmarshal(body, &m); err != nil {
			t.Fatal(err)
		}
		client, ok := m["client"].(map[string]any)
		if !ok || client["first_name"] != "Jane" || client["last_name"] != "Doe" {
			t.Fatalf("body %s", body)
		}
		w.Header().Set("Location", "http://example/api/v1/clients/9")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{"client": map[string]any{"id": 9, "first_name": "Jane", "last_name": "Doe"}},
		})
	}))
	defer srv.Close()

	d := &Dispatcher{Client: apiclient.New(srv.URL, "tok"), AllowWrites: true}
	d.Client.HTTPClient = srv.Client()
	res, err := d.Call(context.Background(), "clients", "create", map[string]any{
		"first_name": "Jane", "last_name": "Doe",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Summary != "client created" {
		t.Fatalf("%+v", res)
	}
}

func TestDispatchArchivePatchPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch || r.URL.Path != "/api/v1/clients/5/archive" {
			t.Fatalf("%s %s", r.Method, r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{"client": map[string]any{"id": 5, "archived_at": "2024-06-01T00:00:00Z"}},
		})
	}))
	defer srv.Close()

	d := &Dispatcher{Client: apiclient.New(srv.URL, "tok"), AllowWrites: true}
	d.Client.HTTPClient = srv.Client()
	res, err := d.Call(context.Background(), "clients", "archive", map[string]any{"id": "5"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Summary, "archived") {
		t.Fatalf("%+v", res)
	}
}

func TestDispatchDestroyRequiresArchiveSemantics(t *testing.T) {
	// Rails: destroy of non-archived → 422 with errors[].message
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Fatalf("method %s", r.Method)
		}
		w.WriteHeader(http.StatusUnprocessableEntity)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"errors": []any{map[string]any{
				"field": "base", "message": "Archive this client before deleting",
			}},
		})
	}))
	defer srv.Close()

	d := &Dispatcher{Client: apiclient.New(srv.URL, "tok"), AllowWrites: true}
	d.Client.HTTPClient = srv.Client()
	_, err := d.Call(context.Background(), "clients", "destroy", map[string]any{"id": "1"})
	ce := output.AsCLIError(err)
	if !strings.Contains(ce.Body.Message, "Archive this client before deleting") {
		t.Fatalf("%v", err)
	}
}

func TestDispatchDestroyNoContent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/api/v1/clients/9" {
			t.Fatalf("%s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	d := &Dispatcher{Client: apiclient.New(srv.URL, "tok"), AllowWrites: true}
	d.Client.HTTPClient = srv.Client()
	res, err := d.Call(context.Background(), "clients", "destroy", map[string]any{"id": "9"})
	if err != nil {
		t.Fatal(err)
	}
	data := res.Data.(map[string]any)
	if data["deleted"] != true || data["id"] != "9" {
		t.Fatalf("%#v", data)
	}
}

func TestDispatchMergeSuccessTopLevelBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/clients/10/merge" {
			t.Fatalf("%s %s", r.Method, r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		var m map[string]any
		_ = json.Unmarshal(body, &m)
		if m["loser_id"] != "11" || m["merge_reason"] != "dup" {
			t.Fatalf("body %s", body)
		}
		// Success shape is top-level (no data wrapper) per controller#merge
		_ = json.NewEncoder(w).Encode(map[string]any{
			"merged": true, "winner_id": 10, "loser_id": 11,
			"moved_record_counts": map[string]any{},
			"rate_limit":         map[string]any{"limit": 100, "remaining": 1, "reset": 5},
		})
	}))
	defer srv.Close()

	d := &Dispatcher{Client: apiclient.New(srv.URL, "tok"), AllowWrites: true}
	d.Client.HTTPClient = srv.Client()
	res, err := d.Call(context.Background(), "clients", "merge", map[string]any{
		"id": "10", "loser_id": "11", "merge_reason": "dup",
	})
	if err != nil {
		t.Fatal(err)
	}
	data := res.Data.(map[string]any)
	if data["merged"] != true {
		t.Fatalf("%#v", data)
	}
	if _, ok := data["rate_limit"]; ok {
		t.Fatalf("rate_limit should be meta only, got data %#v", data)
	}
	if res.Meta["rate_limit"] == nil {
		t.Fatalf("meta %#v", res.Meta)
	}
}

func TestDispatchMergeSingularError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": "Cannot merge a client into itself"})
	}))
	defer srv.Close()

	d := &Dispatcher{Client: apiclient.New(srv.URL, "tok"), AllowWrites: true}
	d.Client.HTTPClient = srv.Client()
	_, err := d.Call(context.Background(), "clients", "merge", map[string]any{"id": "1", "loser_id": "1"})
	ce := output.AsCLIError(err)
	if ce.Body.Message != "Cannot merge a client into itself" {
		t.Fatalf("%v", err)
	}
}

func TestCatalogClientsIncludesDestroy(t *testing.T) {
	d, ok := ByKey("clients")
	if !ok {
		t.Fatal("no clients")
	}
	a, ok := FindAction(d, "destroy")
	if !ok || !a.Implemented || a.Method != "DELETE" || !a.Destructive {
		t.Fatalf("%+v ok=%v", a, ok)
	}
}
