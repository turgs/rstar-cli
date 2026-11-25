package apiclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/turgs/rstar-cli/internal/output"
)

// Contract fixtures mirror ReceptionStar Rails responses proven from:
//   app/controllers/concerns/api_authentication.rb (render_json_error)
//   app/controllers/api/v1/clients_controller.rb
//   app/views/api/v1/clients/{index,show}.json.jb
//   test/integration/api/v1/clients_controller_test.rb
// Live `bin/rails runner` was unavailable on this agent host (bundler path error);
// shapes below are copied from those sources, not invented.

func TestRailsAuthErrorHasFieldMessageNoCode(t *testing.T) {
	// ApiAuthentication#render_json_error → { errors: [{ field, message }] }, status 401
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"errors": []any{map[string]any{
				"field":   "base",
				"message": "Invalid or expired API token",
			}},
		})
	}))
	defer srv.Close()

	c := New(srv.URL, "rs_pat_bad")
	c.HTTPClient = srv.Client()
	_, err := c.Get(context.Background(), "/heart/authed", nil)
	ce := output.AsCLIError(err)
	if ce.Body.Code != output.CodeAuth {
		t.Fatalf("code %q", ce.Body.Code)
	}
	if ce.Body.Message != "Invalid or expired API token" {
		t.Fatalf("message %q", ce.Body.Message)
	}
	if ce.Body.Field != "base" {
		t.Fatalf("field %q", ce.Body.Field)
	}
	if ce.ExitCode != output.ExitAuth {
		t.Fatalf("exit %d", ce.ExitCode)
	}
}

func TestRailsValidationErrors(t *testing.T) {
	// ClientsController#create failure — field/message only, 422
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"errors": []any{
				map[string]any{"field": "first_name", "message": "can't be blank"},
				map[string]any{"field": "last_name", "message": "can't be blank"},
			},
		})
	}))
	defer srv.Close()

	c := New(srv.URL, "tok")
	c.HTTPClient = srv.Client()
	_, err := c.Post(context.Background(), "/clients", nil, map[string]any{
		"client": map[string]any{"first_name": "", "last_name": ""},
	})
	ce := output.AsCLIError(err)
	if ce.Body.Code != output.CodeValidation {
		t.Fatalf("code %q", ce.Body.Code)
	}
	if ce.Body.Message != "can't be blank" {
		t.Fatalf("message %q", ce.Body.Message)
	}
	if ce.Body.Field != "first_name" {
		t.Fatalf("field %q", ce.Body.Field)
	}
}

func TestRailsMergeErrorSingular(t *testing.T) {
	// ClientsController#merge rescue → { error: e.message }, 422
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": "Cannot merge a client into itself",
		})
	}))
	defer srv.Close()

	c := New(srv.URL, "tok")
	c.HTTPClient = srv.Client()
	_, err := c.Post(context.Background(), "/clients/1/merge", nil, map[string]any{"loser_id": "1"})
	ce := output.AsCLIError(err)
	if ce.Body.Message != "Cannot merge a client into itself" {
		t.Fatalf("message %q (singular error key must surface)", ce.Body.Message)
	}
	if ce.Body.Code != output.CodeValidation {
		t.Fatalf("code %q", ce.Body.Code)
	}
}

func TestRailsNotFoundErrorsArray(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"errors": []any{map[string]any{"field": "base", "message": "Record not found"}},
		})
	}))
	defer srv.Close()

	c := New(srv.URL, "tok")
	c.HTTPClient = srv.Client()
	_, err := c.Get(context.Background(), "/clients/nonexistent", nil)
	ce := output.AsCLIError(err)
	if ce.Body.Code != output.CodeNotFound {
		t.Fatalf("code %q", ce.Body.Code)
	}
	if ce.Body.Message != "Record not found" {
		t.Fatalf("message %q", ce.Body.Message)
	}
}

func TestRailsDestroyNoContent(t *testing.T) {
	// destroy success → 204 empty body
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Fatalf("method %s", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := New(srv.URL, "tok")
	c.HTTPClient = srv.Client()
	res, err := c.Delete(context.Background(), "/clients/42", nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != 204 {
		t.Fatalf("status %d", res.StatusCode)
	}
}

func TestRailsIndexShape(t *testing.T) {
	// index.json.jb
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("search") != "Ann" {
			t.Fatalf("search=%q", r.URL.Query().Get("search"))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"clients": []any{
					map[string]any{"id": 1, "first_name": "Ann", "full_name": "Ann A"},
				},
			},
			"pagination": map[string]any{"page": 1, "per_page": 25, "total_count": 1, "total_pages": 1},
			"rate_limit": map[string]any{"limit": 1000, "remaining": 999, "reset": 60},
		})
	}))
	defer srv.Close()

	c := New(srv.URL, "tok")
	c.HTTPClient = srv.Client()
	q := map[string][]string{"search": {"Ann"}}
	res, err := c.Get(context.Background(), "/clients", q)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := res.Data().(map[string]any)
	clients, _ := data["clients"].([]any)
	if len(clients) != 1 {
		t.Fatalf("clients %#v", data)
	}
	if res.Body["pagination"] == nil || res.Body["rate_limit"] == nil {
		t.Fatalf("meta missing: %#v", res.Body)
	}
}

func TestRailsWhoamiShape(t *testing.T) {
	// HeartController#authed
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/heart/authed" {
			t.Fatalf("path %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer rs_pat_x" {
			t.Fatalf("auth %q", r.Header.Get("Authorization"))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data":       map[string]any{"heart": "quote", "user": "a@b.co"},
			"rate_limit": map[string]any{"limit": 100, "remaining": 99, "reset": 30},
		})
	}))
	defer srv.Close()

	c := New(srv.URL, "rs_pat_x")
	c.HTTPClient = srv.Client()
	res, err := c.Get(context.Background(), "/heart/authed", nil)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := res.Data().(map[string]any)
	if data["user"] != "a@b.co" {
		t.Fatalf("%#v", data)
	}
}
