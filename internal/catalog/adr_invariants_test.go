package catalog

import (
	"strings"
	"testing"
)

// ADR 0001 / board turgs/rstar-issues#479 domain-gateway invariants.
// These are executable product rules — fail CI if the catalog drifts.

func TestADR_OneToolPerDomainUniqueKeys(t *testing.T) {
	keys, tools := map[string]bool{}, map[string]bool{}
	for _, d := range All() {
		if d.Key == "" || d.Tool == "" {
			t.Fatalf("domain missing key/tool: %+v", d)
		}
		if !strings.HasPrefix(d.Tool, "rstar_") {
			t.Fatalf("tool %q must be rstar_<domain>", d.Tool)
		}
		if keys[d.Key] {
			t.Fatalf("duplicate domain key %q", d.Key)
		}
		if tools[d.Tool] {
			t.Fatalf("duplicate tool %q", d.Tool)
		}
		keys[d.Key] = true
		tools[d.Tool] = true
		if d.Tool != "rstar_"+d.Key {
			t.Fatalf("tool %q should be rstar_%s", d.Tool, d.Key)
		}
	}
}

func TestADR_DescribeIsSyntheticNotAnActionRow(t *testing.T) {
	// describe is in-band on every tool; it must not appear as a catalog Action
	// (InputSchema appends it separately).
	for _, d := range All() {
		if _, ok := FindAction(d, DescribeAction); ok {
			t.Fatalf("%s lists %q as an Action row — keep it synthetic via DescribeAction", d.Key, DescribeAction)
		}
		schema := d.InputSchema()
		props := schema["properties"].(map[string]any)
		action := props["action"].(map[string]any)
		enum := action["enum"].([]any)
		found := false
		for _, e := range enum {
			if e == DescribeAction {
				found = true
			}
		}
		if !found {
			t.Fatalf("%s InputSchema enum missing %q", d.Key, DescribeAction)
		}
		req := schema["required"].([]any)
		if len(req) == 0 || req[0] != "action" {
			t.Fatalf("%s schema must require action: %#v", d.Key, req)
		}
		if schema["additionalProperties"] != false {
			t.Fatalf("%s schema must set additionalProperties=false", d.Key)
		}
	}
}

func TestADR_ImplementedActionsHaveHTTPContract(t *testing.T) {
	// Local/synthetic implemented actions may omit Method/Path (identity.practices).
	localOK := map[string]bool{
		"identity.practices": true,
	}
	for _, d := range All() {
		for _, a := range d.Actions {
			if !a.Implemented {
				if a.Method != "" || a.Path != "" {
					// roadmap stubs may still list method later; allow empty only
				}
				continue
			}
			id := d.Key + "." + a.Name
			if localOK[id] {
				continue
			}
			if a.Method == "" || a.Path == "" {
				t.Fatalf("%s implemented but missing Method/Path", id)
			}
			if !strings.HasPrefix(a.Path, "/") {
				t.Fatalf("%s path %q must start with /", id, a.Path)
			}
			switch a.Method {
			case "GET", "POST", "PATCH", "PUT", "DELETE":
			default:
				t.Fatalf("%s unknown method %q", id, a.Method)
			}
			if !a.ReadOnly {
				switch a.Method {
				case "GET", "HEAD", "OPTIONS":
					t.Fatalf("%s is write but method %s is read-shaped", id, a.Method)
				}
			}
		}
	}
}

func TestADR_DestructiveImpliesWrite(t *testing.T) {
	for _, d := range All() {
		for _, a := range d.Actions {
			if a.Destructive && a.ReadOnly {
				t.Fatalf("%s.%s destructive cannot be read-only", d.Key, a.Name)
			}
		}
	}
}

func TestADR_FilterUnknownDomainFailsClosed(t *testing.T) {
	_, err := Filter([]string{"identity", "not_a_real_domain"})
	if err == nil {
		t.Fatal("expected fail-closed on unknown domain")
	}
}

func TestADR_FilterReadOnlyDropsWrites(t *testing.T) {
	all := All()
	ro := FilterReadOnly(all)
	for _, d := range ro {
		for _, a := range d.Actions {
			if !a.ReadOnly {
				t.Fatalf("write action leaked: %s.%s", d.Key, a.Name)
			}
		}
	}
	// clients has writes in full catalog
	full, _ := ByKey("clients")
	hasWrite := false
	for _, a := range full.Actions {
		if !a.ReadOnly {
			hasWrite = true
			break
		}
	}
	if !hasWrite {
		t.Fatal("fixture assumption: clients should have write actions")
	}
}

func TestADR_RequiredPathParamsDocumented(t *testing.T) {
	for _, d := range All() {
		for _, a := range d.Actions {
			if !a.Implemented || a.Path == "" {
				continue
			}
			// Every {token} in Path needs a required path param of the same name.
			rest := a.Path
			for {
				i := strings.Index(rest, "{")
				if i < 0 {
					break
				}
				j := strings.Index(rest[i:], "}")
				if j < 0 {
					t.Fatalf("%s.%s unclosed { in path %q", d.Key, a.Name, a.Path)
				}
				token := rest[i+1 : i+j]
				rest = rest[i+j+1:]
				found := false
				for _, p := range a.Params {
					if p.Name == token && p.In == "path" && p.Required {
						found = true
						break
					}
				}
				if !found {
					t.Fatalf("%s.%s path token {%s} needs required path param", d.Key, a.Name, token)
				}
			}
		}
	}
}

func TestADR_VerticalSliceIdentityClientsImplemented(t *testing.T) {
	// Ticket #479 done-looks-like: identity + clients thin slice is live.
	for _, key := range []string{"identity", "clients"} {
		d, ok := ByKey(key)
		if !ok {
			t.Fatalf("missing domain %s", key)
		}
		n := 0
		for _, a := range d.Actions {
			if a.Implemented {
				n++
			}
		}
		if n == 0 {
			t.Fatalf("%s has no implemented actions", key)
		}
	}
}
