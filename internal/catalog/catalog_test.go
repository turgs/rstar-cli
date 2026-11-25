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
