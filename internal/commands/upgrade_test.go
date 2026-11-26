package commands

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/turgs/rstar-cli/internal/release"
	"github.com/turgs/rstar-cli/internal/version"
)

func TestUpgradeDevBuild(t *testing.T) {
	restore := redirectIO(t)
	defer restore()

	old := version.Version
	version.Version = "0.0.0-dev"
	t.Cleanup(func() { version.Version = old })

	flagJSON = true
	defer func() { flagJSON = false }()

	rootCmd.SetArgs([]string{"upgrade"})
	if code := Execute(); code != 0 {
		t.Fatalf("exit %d out=%s err=%s", code, stdout.(*bytes.Buffer).String(), stderr.(*bytes.Buffer).String())
	}
	out := stdout.(*bytes.Buffer).String()
	if !strings.Contains(out, `"status": "dev"`) {
		t.Fatalf("out=%s", out)
	}
}

func TestUpgradeAlreadyLatest(t *testing.T) {
	restore := redirectIO(t)
	defer restore()

	oldV := version.Version
	version.Version = "1.2.3"
	t.Cleanup(func() { version.Version = oldV })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/releases/latest") {
			fmt.Fprint(w, `{"tag_name":"v1.2.3","draft":false,"assets":[]}`)
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)

	oldClient := upgradeReleaseClient
	upgradeReleaseClient = func() *release.Client {
		return &release.Client{HTTP: srv.Client(), BaseURL: srv.URL}
	}
	t.Cleanup(func() { upgradeReleaseClient = oldClient })

	flagJSON = true
	defer func() { flagJSON = false }()

	rootCmd.SetArgs([]string{"upgrade"})
	if code := Execute(); code != 0 {
		t.Fatalf("exit %d out=%s err=%s", code, stdout.(*bytes.Buffer).String(), stderr.(*bytes.Buffer).String())
	}
	out := stdout.(*bytes.Buffer).String()
	if !strings.Contains(out, `"status": "up_to_date"`) {
		t.Fatalf("out=%s", out)
	}
}

func TestUpgradeGoInstallDelegates(t *testing.T) {
	restore := redirectIO(t)
	defer restore()

	oldV := version.Version
	version.Version = "1.0.0"
	t.Cleanup(func() { version.Version = oldV })

	home := t.TempDir()
	goBin := filepath.Join(home, "go", "bin")
	if err := os.MkdirAll(goBin, 0o755); err != nil {
		t.Fatal(err)
	}
	fake := filepath.Join(goBin, "rstar")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\necho rstar 1.0.0\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	// DetectInstallMethod uses real UserHomeDir — only path substring /go/bin/ matters for go_install.
	// Put path with /go/bin/ regardless of home.
	oldExe := upgradeExecutable
	upgradeExecutable = func() (string, error) { return fake, nil }
	t.Cleanup(func() { upgradeExecutable = oldExe })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/releases/latest") {
			fmt.Fprint(w, `{"tag_name":"v1.1.0","draft":false,"assets":[{"name":"checksums.txt","browser_download_url":"http://x","size":1}]}`)
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)

	oldClient := upgradeReleaseClient
	upgradeReleaseClient = func() *release.Client {
		return &release.Client{HTTP: srv.Client(), BaseURL: srv.URL}
	}
	t.Cleanup(func() { upgradeReleaseClient = oldClient })

	flagJSON = true
	defer func() { flagJSON = false }()

	rootCmd.SetArgs([]string{"upgrade"})
	code := Execute()
	if code == 0 {
		t.Fatalf("expected non-zero; out=%s err=%s", stdout.(*bytes.Buffer).String(), stderr.(*bytes.Buffer).String())
	}
	out := stdout.(*bytes.Buffer).String()
	if !strings.Contains(out, "upgrade_required") || !strings.Contains(out, "go install") {
		t.Fatalf("out=%s", out)
	}
}

func TestUpgradeNativeSuccess(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("native swap tested on unix")
	}
	restore := redirectIO(t)
	defer restore()

	oldV := version.Version
	version.Version = "1.0.0"
	t.Cleanup(func() { version.Version = oldV })

	home := t.TempDir()
	// Force PathWithin home by putting exe under home and stubbing is hard —
	// DetectInstallMethod uses os.UserHomeDir. Use a path that looks native under real home.
	realHome, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	binDir := filepath.Join(realHome, ".rstar-upgrade-test-bin")
	_ = os.RemoveAll(binDir)
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(binDir) })

	// Build a real tiny rstar-like stub that answers version for probe,
	// then package it as the "new" release binary with version 1.1.0.
	newPayload := []byte("#!/bin/sh\nif [ \"$1\" = version ]; then\n  if [ \"$2\" = --json ]; then\n    echo '{\"ok\":true,\"data\":{\"version\":\"1.1.0\",\"commit\":\"\"},\"summary\":\"rstar 1.1.0\"}'\n  else\n    echo 'rstar 1.1.0'\n  fi\n  exit 0\nfi\necho unexpected; exit 1\n")
	oldPayload := []byte("#!/bin/sh\necho rstar 1.0.0\n")

	target := filepath.Join(binDir, "rstar")
	if err := os.WriteFile(target, oldPayload, 0o755); err != nil {
		t.Fatal(err)
	}

	archiveName := release.ArchiveName("1.1.0", runtime.GOOS, runtime.GOARCH)
	archiveBytes := mustTarGzBytes(t, release.BinaryName(runtime.GOOS), newPayload)
	sum := sha256Hex(archiveBytes)
	checksums := sum + "  " + archiveName + "\n"

	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/releases/latest"):
			payload := map[string]any{
				"tag_name": "v1.1.0",
				"draft":    false,
				"assets": []map[string]any{
					{
						"name":                 archiveName,
						"browser_download_url": srv.URL + "/download/" + archiveName,
						"size":                 len(archiveBytes),
					},
					{
						"name":                 "checksums.txt",
						"browser_download_url": srv.URL + "/download/checksums.txt",
						"size":                 len(checksums),
					},
				},
			}
			_ = json.NewEncoder(w).Encode(payload)
		case strings.HasSuffix(r.URL.Path, "/download/"+archiveName):
			w.Write(archiveBytes)
		case strings.HasSuffix(r.URL.Path, "/download/checksums.txt"):
			w.Write([]byte(checksums))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	oldClient := upgradeReleaseClient
	upgradeReleaseClient = func() *release.Client {
		return &release.Client{HTTP: srv.Client(), BaseURL: srv.URL}
	}
	t.Cleanup(func() { upgradeReleaseClient = oldClient })

	oldExe := upgradeExecutable
	upgradeExecutable = func() (string, error) { return target, nil }
	t.Cleanup(func() { upgradeExecutable = oldExe })

	// probe uses real exec of the shell stub — fine.
	flagJSON = true
	defer func() { flagJSON = false }()

	rootCmd.SetArgs([]string{"upgrade"})
	code := Execute()
	out := stdout.(*bytes.Buffer).String()
	errOut := stderr.(*bytes.Buffer).String()
	if code != 0 {
		t.Fatalf("exit %d out=%s err=%s", code, out, errOut)
	}
	if !strings.Contains(out, `"status": "upgraded"`) || !strings.Contains(out, "1.1.0") {
		t.Fatalf("out=%s err=%s", out, errOut)
	}
	// target should be new payload
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, newPayload) {
		t.Fatalf("target not updated: %q", got)
	}
	_ = home
}

func TestParseVersionLine(t *testing.T) {
	if got := parseVersionLine("rstar 1.2.3 (abc)\n"); got != "1.2.3" {
		t.Fatal(got)
	}
}

func mustTarGzBytes(t *testing.T, name string, body []byte) []byte {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "a.tar.gz")
	if err := writeTarGzFile(path, name, body); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func writeTarGzFile(path, name string, body []byte) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	hdr := &tar.Header{Name: name, Mode: 0o755, Size: int64(len(body))}
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	if _, err := tw.Write(body); err != nil {
		return err
	}
	if err := tw.Close(); err != nil {
		return err
	}
	return gz.Close()
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
