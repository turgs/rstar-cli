package catalog

import (
	"strings"
	"testing"
)

func TestAllHasIdentityAndClients(t *testing.T) {
	keys := map[string]bool{}
	for _, d := range All() {
		keys[d.Key] = true
		if d.Tool == "" || !strings.HasPrefix(d.Tool, "rstar_") {
			t.Fatalf("bad tool %q on %s", d.Tool, d.Key)
		}
	}
	for _, want := range []string{"identity", "clients", "calendar", "billing"} {
		if !keys[want] {
			t.Fatalf("missing domain %s", want)
		}
	}
}

func TestFilterUnknownFailsClosed(t *testing.T) {
	_, err := Filter([]string{"nope"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestFilterReadOnly(t *testing.T) {
	d, _ := ByKey("clients")
	out := FilterReadOnly([]Domain{d})
	if len(out) != 1 {
		t.Fatal(out)
	}
	for _, a := range out[0].Actions {
		if !a.ReadOnly {
			t.Fatalf("write slipped through: %s", a.Name)
		}
	}
}

func TestDescribe(t *testing.T) {
	d, _ := ByKey("identity")
	payload, err := d.DescribePayload("")
	if err != nil {
		t.Fatal(err)
	}
	m := payload.(map[string]any)
	if m["domain"] != "identity" {
		t.Fatalf("%v", m)
	}
	act, err := d.DescribePayload("whoami")
	if err != nil {
		t.Fatal(err)
	}
	a := act.(Action)
	if a.Name != "whoami" || !a.Implemented {
		t.Fatalf("%+v", a)
	}
}

func TestImplementedClientsActions(t *testing.T) {
	d, _ := ByKey("clients")
	for _, name := range []string{"list", "show", "search", "create"} {
		a, ok := FindAction(d, name)
		if !ok || !a.Implemented {
			t.Fatalf("%s missing or not implemented", name)
		}
	}
}

func TestImplementedCalendarBillingReads(t *testing.T) {
	cal, _ := ByKey("calendar")
	for _, name := range []string{"list", "show", "list_appointment_types"} {
		a, ok := FindAction(cal, name)
		if !ok || !a.Implemented || a.Method == "" || a.Path == "" {
			t.Fatalf("calendar.%s missing binding: %+v ok=%v", name, a, ok)
		}
	}
	for _, name := range []string{"create", "update", "cancel", "uncancel"} {
		a, ok := FindAction(cal, name)
		if !ok || a.Implemented {
			t.Fatalf("calendar.%s should stay not implemented", name)
		}
	}

	bill, _ := ByKey("billing")
	for _, name := range []string{"list_invoices", "show_invoice", "list_unbilled_appointments"} {
		a, ok := FindAction(bill, name)
		if !ok || !a.Implemented || a.Method == "" || a.Path == "" {
			t.Fatalf("billing.%s missing binding: %+v ok=%v", name, a, ok)
		}
	}
	a, ok := FindAction(bill, "invoice_unbilled_appointments")
	if !ok || a.Implemented || !a.Synthetic {
		t.Fatalf("invoice_unbilled write should stay not implemented synthetic: %+v", a)
	}
	unbilled, _ := FindAction(bill, "list_unbilled_appointments")
	if !unbilled.Synthetic {
		t.Fatal("list_unbilled_appointments must stay synthetic")
	}
}
