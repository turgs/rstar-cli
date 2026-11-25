package catalog

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/turgs/rstar-cli/internal/apiclient"
	"github.com/turgs/rstar-cli/internal/output"
)

func TestDispatchDescribeAndWhoami(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/heart/authed" {
			t.Fatalf("path %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{"heart": "x", "user": "a@b.co"},
		})
	}))
	defer srv.Close()

	d := &Dispatcher{
		Client:      apiclient.New(srv.URL, "tok"),
		Profile:     ProfileInfo{ProfileName: "acme", BaseURL: srv.URL},
		AllowWrites: false,
	}
	d.Client.HTTPClient = srv.Client()

	res, err := d.Call(context.Background(), "identity", "describe", nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Data == nil {
		t.Fatal("nil describe")
	}

	res, err = d.Call(context.Background(), "identity", "whoami", nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Summary != "authenticated" {
		t.Fatalf("%+v", res)
	}
}

func TestWriteDenied(t *testing.T) {
	d := &Dispatcher{Client: apiclient.New("http://example", "t"), AllowWrites: false}
	_, err := d.Call(context.Background(), "clients", "create", map[string]any{"first_name": "A", "last_name": "B"})
	ce := output.AsCLIError(err)
	if ce.Body.Code != output.CodeWriteDenied {
		t.Fatalf("got %v", err)
	}
}

func TestUnknownDomainFailsClosed(t *testing.T) {
	d := &Dispatcher{}
	_, err := d.Call(context.Background(), "nope", "list", nil)
	if output.AsCLIError(err).Body.Code != output.CodeDomainDenied {
		t.Fatalf("%v", err)
	}
}

func TestClientsList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("search") != "ann" {
			t.Fatalf("query %v", r.URL.Query())
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data":       map[string]any{"clients": []any{}},
			"pagination": map[string]any{"page": 1, "per_page": 25, "total_count": 0, "total_pages": 0},
		})
	}))
	defer srv.Close()
	d := &Dispatcher{Client: apiclient.New(srv.URL, "t")}
	d.Client.HTTPClient = srv.Client()
	res, err := d.Call(context.Background(), "clients", "search", map[string]any{"q": "ann"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Summary != "clients search" {
		t.Fatalf("%+v", res)
	}
}

func TestNotImplementedRoadmap(t *testing.T) {
	d := &Dispatcher{Client: apiclient.New("http://x", "t"), AllowWrites: false}
	_, err := d.Call(context.Background(), "calendar", "list", nil)
	if output.AsCLIError(err).Body.Code != output.CodeNotImpl {
		t.Fatalf("%v", err)
	}
}
