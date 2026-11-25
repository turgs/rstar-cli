package main

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

func TestRunVersion(t *testing.T) {
	out := captureStdout(t, func() {
		if code := run([]string{"version"}); code != 0 {
			t.Fatalf("exit %d", code)
		}
	})
	if !strings.HasPrefix(out, "rstar ") {
		t.Fatalf("unexpected version output: %q", out)
	}
}

func TestRunDoctor(t *testing.T) {
	out := captureStdout(t, func() {
		if code := run([]string{"doctor"}); code != 0 {
			t.Fatalf("exit %d", code)
		}
	})
	if !strings.Contains(out, "rstar doctor: ok") {
		t.Fatalf("unexpected doctor output: %q", out)
	}
}

func TestRunUnknown(t *testing.T) {
	code := run([]string{"not-a-command"})
	if code != 2 {
		t.Fatalf("want exit 2, got %d", code)
	}
}

func TestRunHelp(t *testing.T) {
	out := captureStdout(t, func() {
		if code := run([]string{"help"}); code != 0 {
			t.Fatalf("exit %d", code)
		}
	})
	if !strings.Contains(out, "Usage:") {
		t.Fatalf("help missing Usage: %q", out)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	fn()
	_ = w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatal(err)
	}
	_ = r.Close()
	return buf.String()
}
