package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeBaseURL(t *testing.T) {
	cases := map[string]string{
		"https://acme.receptionstar.com/":        "https://acme.receptionstar.com",
		"https://acme.receptionstar.com/api/v1":  "https://acme.receptionstar.com",
		"https://acme.receptionstar.com/api/v1/": "https://acme.receptionstar.com",
		" http://localhost:3001 ":                "http://localhost:3001",
	}
	for in, want := range cases {
		if got := NormalizeBaseURL(in); got != want {
			t.Errorf("NormalizeBaseURL(%q)=%q want %q", in, got, want)
		}
	}
}

func TestUpsertAndLoad(t *testing.T) {
	dir := t.TempDir()
	SetTestConfigDir(dir)
	t.Cleanup(ResetTestConfigDir)
	t.Setenv(EnvBaseURL, "")
	t.Setenv(EnvToken, "")
	t.Setenv(EnvProfile, "")

	if err := UpsertProfile("acme", "https://acme.receptionstar.com/api/v1/", true); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "config.yaml")
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(ResolveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ProfileName != "acme" {
		t.Fatalf("profile %q", cfg.ProfileName)
	}
	if cfg.BaseURL != "https://acme.receptionstar.com" {
		t.Fatalf("base %q", cfg.BaseURL)
	}
	if cfg.BaseURLSource != "profile" {
		t.Fatalf("source %q", cfg.BaseURLSource)
	}
}

func TestEnvOverridesProfile(t *testing.T) {
	dir := t.TempDir()
	SetTestConfigDir(dir)
	t.Cleanup(ResetTestConfigDir)
	_ = UpsertProfile("acme", "https://acme.receptionstar.com", true)
	t.Setenv(EnvBaseURL, "https://other.example.com")
	t.Setenv(EnvToken, "rs_pat_test")

	cfg, err := Load(ResolveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.BaseURL != "https://other.example.com" || cfg.BaseURLSource != "env" {
		t.Fatalf("base override: %+v", cfg)
	}
	if cfg.Token != "rs_pat_test" || cfg.TokenSource != "env" {
		t.Fatalf("token override: %+v", cfg)
	}
}

func TestValidateProfileName(t *testing.T) {
	if err := ValidateProfileName("acme-1"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateProfileName("bad name"); err == nil {
		t.Fatal("expected error")
	}
}

func TestValidateBaseURL(t *testing.T) {
	if err := ValidateBaseURL("https://acme.receptionstar.com"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateBaseURL("http://localhost:3001"); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"", "not-a-url", "example.com", "ftp://x", "//evil.com", "https://user:pass@host"} {
		if err := ValidateBaseURL(bad); err == nil {
			t.Fatalf("expected error for %q", bad)
		}
	}
}

func TestUpsertRejectsBadBaseURL(t *testing.T) {
	dir := t.TempDir()
	SetTestConfigDir(dir)
	t.Cleanup(ResetTestConfigDir)
	if err := UpsertProfile("acme", "not-a-url", true); err == nil {
		t.Fatal("expected error")
	}
}
