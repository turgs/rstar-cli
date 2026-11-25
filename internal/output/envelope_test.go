package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestWriteSuccess(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteSuccess(&buf, map[string]any{"id": "1"}, "ok", Breadcrumb{Label: "next", Command: "rstar clients show 1"}); err != nil {
		t.Fatal(err)
	}
	var env Envelope
	if err := json.Unmarshal(buf.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if !env.OK || env.Summary != "ok" || len(env.Breadcrumbs) != 1 {
		t.Fatalf("bad envelope: %+v", env)
	}
}

func TestWriteError(t *testing.T) {
	var buf bytes.Buffer
	err := ErrAuth("no token").WithRetryable(false)
	if werr := WriteError(&buf, err); werr != nil {
		t.Fatal(werr)
	}
	if !strings.Contains(buf.String(), `"ok": false`) {
		t.Fatalf("want ok false, got %s", buf.String())
	}
	if !strings.Contains(buf.String(), CodeAuth) {
		t.Fatalf("want auth code, got %s", buf.String())
	}
}

func TestExitCodeFor(t *testing.T) {
	if ExitCodeFor(nil) != ExitOK {
		t.Fatal("nil")
	}
	if ExitCodeFor(ErrAuth("x")) != ExitAuth {
		t.Fatal("auth")
	}
}
