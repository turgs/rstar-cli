package auth

import (
	"testing"
)

func TestFileRoundTrip(t *testing.T) {
	dir := t.TempDir()
	SetTestDir(dir)
	t.Cleanup(ResetTestDir)

	s := &Store{PreferKeyring: false}
	storage, err := s.Set("acme", "rs_pat_secret")
	if err != nil {
		t.Fatal(err)
	}
	if storage != "file" {
		t.Fatalf("storage %q", storage)
	}
	tok, st, err := s.Get("acme")
	if err != nil {
		t.Fatal(err)
	}
	if tok != "rs_pat_secret" || st != "file" {
		t.Fatalf("got %q %q", tok, st)
	}
	if err := s.Delete("acme"); err != nil {
		t.Fatal(err)
	}
	tok, _, err = s.Get("acme")
	if err != nil {
		t.Fatal(err)
	}
	if tok != "" {
		t.Fatalf("want empty after delete, got %q", tok)
	}
}

func TestRejectTokenWithNewline(t *testing.T) {
	s := &Store{PreferKeyring: false, FileDir: t.TempDir()}
	if _, err := s.Set("acme", "rs_pat_x\nsmuggled"); err == nil {
		t.Fatal("expected error")
	}
}
