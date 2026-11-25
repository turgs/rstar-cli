package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/turgs/rstar-cli/internal/auth"
	"github.com/turgs/rstar-cli/internal/config"
)

func TestCommandsJSON(t *testing.T) {
	restore := redirectIO(t)
	defer restore()

	flagJSON = true
	defer func() { flagJSON = false }()

	rootCmd.SetArgs([]string{"commands", "--json"})
	code := Execute()
	if code != 0 {
		t.Fatalf("exit %d out=%s err=%s", code, stdout.(*bytes.Buffer).String(), stderr.(*bytes.Buffer).String())
	}
	out := stdout.(*bytes.Buffer).String()
	if !strings.Contains(out, `"ok": true`) || !strings.Contains(out, "rstar_identity") {
		t.Fatalf("out=%s", out)
	}
}

func TestAuthLoginAndStatus(t *testing.T) {
	dir := t.TempDir()
	config.SetTestConfigDir(dir)
	t.Cleanup(config.ResetTestConfigDir)
	auth.SetTestDir(dir)
	t.Cleanup(auth.ResetTestDir)
	authStore = &auth.Store{PreferKeyring: false, FileDir: dir}

	restore := redirectIO(t)
	defer restore()

	flagJSON = true
	defer func() { flagJSON = false }()

	rootCmd.SetArgs([]string{"auth", "login", "--base-url", "https://acme.example.com", "rs_pat_x"})
	if code := Execute(); code != 0 {
		t.Fatalf("login exit %d: %s", code, stderr.(*bytes.Buffer).String())
	}
	stdout.(*bytes.Buffer).Reset()
	stderr.(*bytes.Buffer).Reset()

	rootCmd.SetArgs([]string{"auth", "status"})
	if code := Execute(); code != 0 {
		t.Fatalf("status exit %d", code)
	}
	out := stdout.(*bytes.Buffer).String()
	if !strings.Contains(out, `"authenticated": true`) {
		t.Fatalf("status %s", out)
	}

	// config file exists
	if _, err := os.Stat(filepath.Join(dir, "config.yaml")); err != nil {
		t.Fatal(err)
	}
}

func TestHelpAgent(t *testing.T) {
	restore := redirectIO(t)
	defer restore()
	flagJSON = false
	rootCmd.SetArgs([]string{"help", "--agent"})
	if code := Execute(); code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(stdout.(*bytes.Buffer).String(), "PHI") {
		t.Fatalf("%s", stdout.(*bytes.Buffer).String())
	}
}

func redirectIO(t *testing.T) func() {
	t.Helper()
	oldOut, oldErr := stdout, stderr
	ob, eb := &bytes.Buffer{}, &bytes.Buffer{}
	stdout, stderr = ob, eb
	rootCmd.SetOut(ob)
	rootCmd.SetErr(eb)
	return func() {
		stdout, stderr = oldOut, oldErr
		rootCmd.SetOut(oldOut)
		rootCmd.SetErr(oldErr)
		rootCmd.SetArgs(nil)
	}
}
