package release

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestParseChecksums(t *testing.T) {
	data := []byte("aabbccdd00112233445566778899aabbccdd00112233445566778899aabbccdd  rstar_1.0.0_linux_amd64.tar.gz\n" +
		"# comment\n" +
		"bbccddee00112233445566778899aabbccdd00112233445566778899aabbccdd *rstar_1.0.0_darwin_arm64.tar.gz\n")
	m := ParseChecksums(data)
	if m["rstar_1.0.0_linux_amd64.tar.gz"] != "aabbccdd00112233445566778899aabbccdd00112233445566778899aabbccdd" {
		t.Fatalf("%v", m)
	}
	if m["rstar_1.0.0_darwin_arm64.tar.gz"] != "bbccddee00112233445566778899aabbccdd00112233445566778899aabbccdd" {
		t.Fatalf("%v", m)
	}
}

func TestIsUpdateAvailable(t *testing.T) {
	cases := []struct {
		cur, lat string
		want     bool
	}{
		{"1.0.0", "1.0.1", true},
		{"v1.0.1", "1.0.1", false},
		{"1.0.1", "1.0.0", false},
		{"0.0.0-dev", "1.0.0", false},
		{"1.0.0-rc.1", "1.0.0", true},
	}
	for _, tc := range cases {
		if got := IsUpdateAvailable(tc.cur, tc.lat); got != tc.want {
			t.Errorf("IsUpdateAvailable(%q,%q)=%v want %v", tc.cur, tc.lat, got, tc.want)
		}
	}
}

func TestIsDevBuild(t *testing.T) {
	if !IsDevBuild("0.0.0-dev") || !IsDevBuild("dev") || IsDevBuild("1.2.3") {
		t.Fatal("dev detection")
	}
}

func TestArchiveName(t *testing.T) {
	if got := ArchiveName("1.2.3", "linux", "amd64"); got != "rstar_1.2.3_linux_amd64.tar.gz" {
		t.Fatal(got)
	}
	if got := ArchiveName("v1.2.3", "windows", "arm64"); got != "rstar_1.2.3_windows_arm64.zip" {
		t.Fatal(got)
	}
}

func TestClientLatest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/turgs/rstar-cli/releases/latest" {
			http.NotFound(w, r)
			return
		}
		fmt.Fprint(w, `{"tag_name":"v1.2.3","name":"v1.2.3","draft":false,"assets":[{"name":"checksums.txt","browser_download_url":"http://example/checksums.txt","size":10}]}`)
	}))
	defer srv.Close()

	c := &Client{HTTP: srv.Client(), BaseURL: srv.URL}
	info, err := c.Latest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if info.Version() != "1.2.3" {
		t.Fatalf("%+v", info)
	}
	if _, ok := info.AssetByName("checksums.txt"); !ok {
		t.Fatal("missing asset")
	}
}

func TestDetectInstallMethod(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		path string
		want Method
	}{
		{"/opt/homebrew/bin/rstar", MethodHomebrew},
		{"/home/linuxbrew/.linuxbrew/Cellar/rstar/1.0.0/bin/rstar", MethodHomebrew},
		{"/nix/store/abc-rstar/bin/rstar", MethodNix},
		{filepath.Join(home, "go", "bin", "rstar"), MethodGoInstall},
		{filepath.Join(home, ".local", "bin", "rstar"), MethodUserNative},
		{"/usr/local/bin/rstar", MethodSystem},
	}
	for _, tc := range cases {
		if strings.HasPrefix(tc.path, home) {
			_ = os.MkdirAll(filepath.Dir(tc.path), 0o755)
			if _, err := os.Stat(tc.path); err != nil {
				if f, err := os.Create(tc.path); err == nil {
					_ = f.Close()
					t.Cleanup(func() { _ = os.Remove(tc.path) })
				}
			}
		}
		if got := DetectInstallMethod(tc.path); got != tc.want {
			t.Errorf("DetectInstallMethod(%q)=%s want %s", tc.path, got, tc.want)
		}
	}
}

func TestPathWithin(t *testing.T) {
	root := t.TempDir()
	inside := filepath.Join(root, "bin", "rstar")
	_ = os.MkdirAll(filepath.Dir(inside), 0o755)
	if err := os.WriteFile(inside, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !PathWithin(root, inside) {
		t.Fatal("expected inside")
	}
	outside := filepath.Join(t.TempDir(), "other")
	if err := os.WriteFile(outside, []byte("y"), 0o644); err != nil {
		t.Fatal(err)
	}
	if PathWithin(root, outside) {
		t.Fatal("outside should not match")
	}
}

func TestExtractAndReplace(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("archive helpers exercised on unix CI")
	}
	dir := t.TempDir()
	payload := []byte("#!/bin/sh\necho fake\n")
	archive := filepath.Join(dir, "rstar_1.0.0_linux_amd64.tar.gz")
	if err := writeTarGz(archive, "rstar", payload); err != nil {
		t.Fatal(err)
	}

	staged := filepath.Join(dir, "staged")
	if err := ExtractBinary(archive, "tar.gz", "rstar", staged); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(staged)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(payload) {
		t.Fatalf("extracted %q", got)
	}

	target := filepath.Join(dir, "install", "rstar")
	_ = os.MkdirAll(filepath.Dir(target), 0o755)
	if err := os.WriteFile(target, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	bak, err := ReplaceExecutable(target, staged)
	if err != nil {
		t.Fatal(err)
	}
	got, err = os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(payload) {
		t.Fatal("target not replaced")
	}
	if bak == "" {
		t.Fatal("expected backup")
	}
	old, err := os.ReadFile(bak)
	if err != nil || string(old) != "old" {
		t.Fatalf("backup %q err=%v", bak, err)
	}
	DiscardBackup(bak)
}

func writeTarGz(path, name string, body []byte) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	hdr := &tar.Header{
		Name: name,
		Mode: 0o755,
		Size: int64(len(body)),
	}
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
