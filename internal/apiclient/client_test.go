package apiclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/turgs/rstar-cli/internal/output"
)

func TestGetSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer rs_pat_x" {
			t.Errorf("auth header %q", r.Header.Get("Authorization"))
		}
		if r.URL.Path != "/api/v1/clients" {
			t.Errorf("path %q", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data":       map[string]any{"clients": []any{}},
			"pagination": map[string]any{"page": 1, "per_page": 25, "total_count": 0, "total_pages": 0},
		})
	}))
	defer srv.Close()

	c := New(srv.URL, "rs_pat_x")
	c.HTTPClient = srv.Client()
	res, err := c.Get(context.Background(), "/clients", nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != 200 {
		t.Fatalf("status %d", res.StatusCode)
	}
	data, _ := res.Data().(map[string]any)
	if data == nil {
		t.Fatalf("data: %#v", res.Body)
	}
}

func TestGetUnauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"errors": []any{map[string]any{"field": "base", "message": "bad token", "code": "unauthorized", "retryable": false}},
		})
	}))
	defer srv.Close()

	c := New(srv.URL, "bad")
	c.HTTPClient = srv.Client()
	_, err := c.Get(context.Background(), "/heart/authed", nil)
	ce := output.AsCLIError(err)
	if ce.Body.Code != output.CodeAuth {
		t.Fatalf("code %q", ce.Body.Code)
	}
	if ce.ExitCode != output.ExitAuth {
		t.Fatalf("exit %d", ce.ExitCode)
	}
}

func TestMissingConfig(t *testing.T) {
	c := New("", "")
	_, err := c.Get(context.Background(), "/clients", nil)
	if output.AsCLIError(err).Body.Code != output.CodeConfig {
		t.Fatalf("got %v", err)
	}
}

func TestInvalidBaseURL(t *testing.T) {
	c := New("not-a-url", "tok")
	_, err := c.Get(context.Background(), "/clients", nil)
	if output.AsCLIError(err).Body.Code != output.CodeConfig {
		t.Fatalf("got %v", err)
	}
}

func TestResolveRejectsDotDot(t *testing.T) {
	c := New("https://example.com", "tok")
	_, err := c.resolve("/clients/../heart", nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestResolveRejectsEncodedDotDot(t *testing.T) {
	c := New("https://example.com", "tok")
	// path.Clean after parse must catch %2e%2e walking above /api/v1.
	_, err := c.resolve("/clients/%2e%2e/%2e%2e/secret", nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestResolveJoin(t *testing.T) {
	c := New("https://example.com", "tok")
	got, err := c.resolve("/clients", nil)
	if err != nil {
		t.Fatal(err)
	}
	if got != "https://example.com/api/v1/clients" {
		t.Fatalf("got %q", got)
	}
}

func TestRedirectRefused(t *testing.T) {
	final := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not follow redirect")
	}))
	defer final.Close()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, final.URL+"/api/v1/clients", http.StatusFound)
	}))
	defer srv.Close()

	c := New(srv.URL, "tok")
	// Keep CheckRedirect; use a plain transport so only our client policy applies.
	c.HTTPClient.Transport = http.DefaultTransport
	_, err := c.Get(context.Background(), "/clients", nil)
	if err == nil {
		t.Fatal("expected redirect error")
	}
	if output.AsCLIError(err).Body.Code != output.CodeNetwork {
		t.Fatalf("got %v", err)
	}
}
