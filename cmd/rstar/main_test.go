package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestBinarySmoke(t *testing.T) {
	bin := buildTestBinary(t)

	out, err := exec.Command(bin, "version").CombinedOutput()
	if err != nil {
		t.Fatalf("version: %v %s", err, out)
	}
	if !strings.Contains(string(out), "rstar ") {
		t.Fatalf("version output: %s", out)
	}

	out, err = exec.Command(bin, "help").CombinedOutput()
	if err != nil {
		t.Fatalf("help: %v %s", err, out)
	}
	if !strings.Contains(string(out), "ReceptionStar") {
		t.Fatalf("help: %s", out)
	}

	out, err = exec.Command(bin, "help", "--agent").CombinedOutput()
	if err != nil {
		t.Fatalf("help --agent: %v %s", err, out)
	}
	if !strings.Contains(string(out), "Envelope") {
		t.Fatalf("agent help: %s", out)
	}

	// Ticket acceptance: --help --agent (persistent flag + HelpFunc), not only help --agent.
	out, err = exec.Command(bin, "--help", "--agent").CombinedOutput()
	if err != nil {
		t.Fatalf("--help --agent: %v %s", err, out)
	}
	if !strings.Contains(string(out), "Envelope") || !strings.Contains(string(out), "PHI") {
		t.Fatalf("--help --agent: %s", out)
	}

	out, err = exec.Command(bin, "commands", "--json").CombinedOutput()
	if err != nil {
		t.Fatalf("commands: %v %s", err, out)
	}
	if !strings.Contains(string(out), `"ok": true`) {
		t.Fatalf("commands json: %s", out)
	}

	dir := t.TempDir()
	cmd := exec.Command(bin, "doctor", "--json")
	cmd.Env = append(os.Environ(), "RSTAR_CONFIG_DIR="+dir, "RSTAR_BASE_URL=", "RSTAR_TOKEN=", "RSTAR_PROFILE=")
	out, err = cmd.CombinedOutput()
	// doctor fails without auth — exit non-zero is expected
	if !strings.Contains(string(out), "checks") {
		t.Fatalf("doctor: err=%v out=%s", err, out)
	}

	out, err = exec.Command(bin, "skill", "show").CombinedOutput()
	if err != nil {
		t.Fatalf("skill show: %v %s", err, out)
	}
	if !strings.Contains(string(out), "Agent invariants") {
		t.Fatalf("skill: %s", out)
	}
}

func buildTestBinary(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "rstar")
	cmd := exec.Command("go", "build", "-o", bin, ".")
	cmd.Dir = filepath.Join() // this package
	// run from module root
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	cmd = exec.Command("go", "build", "-o", bin, "./cmd/rstar")
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build: %v %s", err, out)
	}
	return bin
}
