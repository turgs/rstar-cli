package catalog

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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
	_, err := d.Call(context.Background(), "clinical", "list_case_notes", nil)
	if output.AsCLIError(err).Body.Code != output.CodeNotImpl {
		t.Fatalf("%v", err)
	}
	// Calendar writes stay off this slice.
	_, err = d.Call(context.Background(), "calendar", "create", map[string]any{})
	if output.AsCLIError(err).Body.Code != output.CodeWriteDenied {
		t.Fatalf("expected write denied without --write, got %v", err)
	}
	d.AllowWrites = true
	_, err = d.Call(context.Background(), "calendar", "create", map[string]any{})
	if output.AsCLIError(err).Body.Code != output.CodeNotImpl {
		t.Fatalf("expected not_impl for calendar.create, got %v", err)
	}
}

func TestCalendarListDefaultsIsAppointment(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/engagements" {
			t.Fatalf("path %s", r.URL.Path)
		}
		if r.URL.Query().Get("is_appointment") != "true" {
			t.Fatalf("query %v", r.URL.Query())
		}
		if r.URL.Query().Get("from_date") != "2026-01-01" {
			t.Fatalf("from_date %v", r.URL.Query())
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data":       map[string]any{"engagements": []any{}},
			"pagination": map[string]any{"page": 1, "per_page": 25, "total_count": 0, "total_pages": 0},
		})
	}))
	defer srv.Close()
	d := &Dispatcher{Client: apiclient.New(srv.URL, "t")}
	d.Client.HTTPClient = srv.Client()
	res, err := d.Call(context.Background(), "calendar", "list", map[string]any{"from_date": "2026-01-01"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Summary != "appointments listed" {
		t.Fatalf("%+v", res)
	}
	data := res.Data.(map[string]any)
	if _, ok := data["engagements"]; !ok {
		t.Fatalf("%#v", data)
	}
}

// MCP JSON unmarshals booleans as bool; must not be treated as "unset" (hosted had a blank? bug).
func TestCalendarListBoolFalseIsAppointment(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("is_appointment") != "false" {
			t.Fatalf("query %v", r.URL.Query())
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{"engagements": []any{}},
		})
	}))
	defer srv.Close()
	d := &Dispatcher{Client: apiclient.New(srv.URL, "t")}
	d.Client.HTTPClient = srv.Client()
	_, err := d.Call(context.Background(), "calendar", "list", map[string]any{"is_appointment": false})
	if err != nil {
		t.Fatal(err)
	}
}

// Encoded slash / .. in ids must fail closed in the client (not rewrite to /api/v1/admin).
func TestCalendarShowPathInjectionRejected(t *testing.T) {
	d := &Dispatcher{Client: apiclient.New("https://example.com", "t")}
	for _, id := range []string{"../admin", "..%2Fadmin", "a/b", "a%2Fb", `..\admin`} {
		_, err := d.Call(context.Background(), "calendar", "show", map[string]any{"id": id})
		if err == nil {
			t.Fatalf("expected reject for id %q", id)
		}
		ce := output.AsCLIError(err)
		if ce.Body.Code != output.CodeUsage {
			t.Fatalf("id %q: want usage, got %v", id, err)
		}
		if !strings.Contains(ce.Body.Message, "invalid path") {
			t.Fatalf("id %q: message %q", id, ce.Body.Message)
		}
	}
}

func TestBillingShowPathInjectionRejected(t *testing.T) {
	d := &Dispatcher{Client: apiclient.New("https://example.com", "t")}
	_, err := d.Call(context.Background(), "billing", "show_invoice", map[string]any{"id": "a/b"})
	if err == nil {
		t.Fatal("expected reject")
	}
	if output.AsCLIError(err).Body.Code != output.CodeUsage {
		t.Fatalf("got %v", err)
	}
}

func TestCalendarListRejectsUnknownQueryPollution(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		for _, evil := range []string{"host", "url", "authorization", "path", "method"} {
			if q.Get(evil) != "" {
				t.Fatalf("smuggled %s=%q full=%v", evil, q.Get(evil), q)
			}
		}
		if q.Get("is_appointment") != "true" {
			t.Fatalf("default missing: %v", q)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"engagements": []any{}}})
	}))
	defer srv.Close()
	d := &Dispatcher{Client: apiclient.New(srv.URL, "t")}
	d.Client.HTTPClient = srv.Client()
	_, err := d.Call(context.Background(), "calendar", "list", map[string]any{
		"host": "evil.example", "url": "http://169.254.169.254/", "authorization": "Bearer x",
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestCalendarShow(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/engagements/eng1" {
			t.Fatalf("path %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{"engagement": map[string]any{"id": "eng1", "is_appointment": true}},
		})
	}))
	defer srv.Close()
	d := &Dispatcher{Client: apiclient.New(srv.URL, "t")}
	d.Client.HTTPClient = srv.Client()
	res, err := d.Call(context.Background(), "calendar", "show", map[string]any{"id": "eng1"})
	if err != nil {
		t.Fatal(err)
	}
	eng := res.Data.(map[string]any)["engagement"].(map[string]any)
	if eng["id"] != "eng1" {
		t.Fatalf("%#v", eng)
	}
}

func TestCalendarListAppointmentTypes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/appointment_types" {
			t.Fatalf("path %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{"appointment_types": []any{map[string]any{"id": "t1", "name": "Initial"}}},
		})
	}))
	defer srv.Close()
	d := &Dispatcher{Client: apiclient.New(srv.URL, "t")}
	d.Client.HTTPClient = srv.Client()
	res, err := d.Call(context.Background(), "calendar", "list_appointment_types", nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Summary != "appointment types listed" {
		t.Fatalf("%+v", res)
	}
}

func TestBillingListAndShowInvoice(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v1/invoices" && r.Method == http.MethodGet:
			if r.URL.Query().Get("client_id") != "c1" {
				t.Fatalf("query %v", r.URL.Query())
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data":       map[string]any{"invoices": []any{}},
				"pagination": map[string]any{"page": 1, "per_page": 25, "total_count": 0, "total_pages": 0},
			})
		case r.URL.Path == "/api/v1/invoices/inv1":
			if r.URL.Query().Get("view") != "summary" {
				t.Fatalf("query %v", r.URL.Query())
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{"invoice": map[string]any{"id": "inv1", "total_amount": 100}},
			})
		case r.URL.Path == "/api/v1/invoices/unbilled_appointments":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{"clients": []any{}},
			})
		default:
			t.Fatalf("%s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()
	d := &Dispatcher{Client: apiclient.New(srv.URL, "t")}
	d.Client.HTTPClient = srv.Client()

	res, err := d.Call(context.Background(), "billing", "list_invoices", map[string]any{"client_id": "c1"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Summary != "invoices listed" {
		t.Fatalf("%+v", res)
	}

	res, err = d.Call(context.Background(), "billing", "show_invoice", map[string]any{"id": "inv1", "view": "summary"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Data.(map[string]any)["invoice"].(map[string]any)["id"] != "inv1" {
		t.Fatalf("%#v", res.Data)
	}

	res, err = d.Call(context.Background(), "billing", "list_unbilled_appointments", nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Summary != "unbilled appointments listed" {
		t.Fatalf("%+v", res)
	}
}
