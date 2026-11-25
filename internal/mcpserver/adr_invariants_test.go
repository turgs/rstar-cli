package mcpserver

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/turgs/rstar-cli/internal/apiclient"
	"github.com/turgs/rstar-cli/internal/catalog"
)

// ADR 0001 MCP surface: one tool per domain, read-only default, no CLI exec.

func TestADR_DefaultReadOnly(t *testing.T) {
	c := apiclient.New("https://example.com", "tok")
	s, err := New(c, catalog.ProfileInfo{}, Config{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	if !s.readOnly {
		t.Fatal("ReadOnly true must set server readOnly")
	}
	if s.dispatch.AllowWrites {
		t.Fatal("read-only server must not AllowWrites")
	}
	for _, d := range s.Domains() {
		for _, a := range d.Actions {
			if !a.ReadOnly {
				t.Fatalf("write action served in read-only mode: %s.%s", d.Key, a.Name)
			}
		}
	}
}

func TestADR_OneMCPToolPerDomain(t *testing.T) {
	c := apiclient.New("https://example.com", "tok")
	s, err := New(c, catalog.ProfileInfo{}, Config{ReadOnly: true, Domains: []string{"identity", "clients"}})
	if err != nil {
		t.Fatal(err)
	}
	tools := map[string]bool{}
	for _, d := range s.Domains() {
		if tools[d.Tool] {
			t.Fatalf("duplicate tool %s", d.Tool)
		}
		tools[d.Tool] = true
	}
	if len(tools) != 2 {
		t.Fatalf("want 2 tools, got %d", len(tools))
	}
}

func TestADR_NoExecOfCLIInMCPPackage(t *testing.T) {
	// Product non-goal: MCP must not shell out to the rstar binary.
	// AST-scan this package for os/exec imports (tests may use exec elsewhere).
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("no caller")
	}
	dir := filepath.Dir(file)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		path := filepath.Join(dir, name)
		f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, imp := range f.Imports {
			p := strings.Trim(imp.Path.Value, `"`)
			if p == "os/exec" || p == "os/exec/v2" {
				t.Fatalf("%s imports %s — MCP must not exec CLI subprocesses", name, p)
			}
		}
		// Also reject bare exec.Command identifiers via full parse if import sneaks in.
		f2, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(f2, func(n ast.Node) bool {
			se, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			id, ok := se.X.(*ast.Ident)
			if ok && id.Name == "exec" && (se.Sel.Name == "Command" || se.Sel.Name == "CommandContext") {
				t.Errorf("%s calls exec.%s — forbidden in mcpserver production code", name, se.Sel.Name)
			}
			return true
		})
	}
}

func TestADR_WritesOptIn(t *testing.T) {
	c := apiclient.New("https://example.com", "tok")
	s, err := New(c, catalog.ProfileInfo{}, Config{ReadOnly: false, Domains: []string{"clients"}})
	if err != nil {
		t.Fatal(err)
	}
	if !s.dispatch.AllowWrites {
		t.Fatal("ReadOnly false should AllowWrites")
	}
	foundWrite := false
	for _, d := range s.Domains() {
		for _, a := range d.Actions {
			if !a.ReadOnly {
				foundWrite = true
			}
		}
	}
	if !foundWrite {
		t.Fatal("writes mode should expose at least one write action on clients")
	}
}
