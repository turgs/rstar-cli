package catalog

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/turgs/rstar-cli/internal/apiclient"
)

// Rails contract: EngagementsController#index filters + Invoices unbilled path.
// Sources: ReceptionStar app/controllers/api/v1/engagements_controller.rb
//          app/controllers/api/v1/invoices/unbilled_appointments_controller.rb

func TestRailsEngagementsListFilters(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/engagements" {
			t.Fatalf("path %s", r.URL.Path)
		}
		q := r.URL.Query()
		// API accepts only string true/false for is_appointment
		if q.Get("is_appointment") != "true" {
			t.Fatalf("is_appointment %q", q.Get("is_appointment"))
		}
		if q.Get("from_date") != "2026-01-15" {
			t.Fatalf("from_date %q", q.Get("from_date"))
		}
		if q.Get("client_ids") != "c1,c2" {
			t.Fatalf("client_ids %q", q.Get("client_ids"))
		}
		if q.Get("sort") != "start_date:asc" {
			t.Fatalf("sort %q", q.Get("sort"))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data":       map[string]any{"engagements": []any{}},
			"pagination": map[string]any{"page": 1, "per_page": 25, "total_count": 0, "total_pages": 0},
		})
	}))
	defer srv.Close()
	d := &Dispatcher{Client: apiclient.New(srv.URL, "t")}
	d.Client.HTTPClient = srv.Client()
	_, err := d.Call(context.Background(), "calendar", "list", map[string]any{
		"from":       "2026-01-15", // alias
		"client_ids": "c1,c2",
		"sort":       "start_date:asc",
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestRailsUnbilledAppointmentsPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/invoices/unbilled_appointments" {
			t.Fatalf("%s %s", r.Method, r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{"synthetic": true, "clients": []any{}},
		})
	}))
	defer srv.Close()
	d := &Dispatcher{Client: apiclient.New(srv.URL, "t")}
	d.Client.HTTPClient = srv.Client()
	res, err := d.Call(context.Background(), "billing", "list_unbilled_appointments", nil)
	if err != nil {
		t.Fatal(err)
	}
	data := res.Data.(map[string]any)
	if data["synthetic"] != true {
		t.Fatalf("%#v", data)
	}
}
