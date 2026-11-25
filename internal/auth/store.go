// Package auth stores staff PATs in the OS keyring with a file fallback.
package auth

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/zalando/go-keyring"

	"github.com/turgs/rstar-cli/internal/config"
)

const (
	keyringService = "rstar-cli"
	fileName       = "credentials"
)

// Store reads and writes PATs per profile.
type Store struct {
	// PreferKeyring tries the OS keyring first. Tests may set false.
	PreferKeyring bool
	// FileDir overrides the credentials directory (defaults to config dir).
	FileDir string
}

var (
	testDir string
	testMu  sync.Mutex
)

// SetTestDir points the file fallback at a temp dir.
func SetTestDir(dir string) {
	testMu.Lock()
	defer testMu.Unlock()
	testDir = dir
}

// ResetTestDir clears the test override.
func ResetTestDir() {
	SetTestDir("")
}

// Default returns the production store.
func Default() *Store {
	return &Store{PreferKeyring: true}
}

func (s *Store) dir() (string, error) {
	testMu.Lock()
	override := testDir
	testMu.Unlock()
	if override != "" {
		return override, nil
	}
	if s.FileDir != "" {
		return s.FileDir, nil
	}
	return config.Dir()
}

func keyringUser(profile string) string {
	return "profile:" + profile
}

// Set saves a PAT for profile.
func (s *Store) Set(profile, token string) (storage string, err error) {
	profile = strings.TrimSpace(profile)
	token = strings.TrimSpace(token)
	if profile == "" {
		return "", errors.New("profile is required")
	}
	if token == "" {
		return "", errors.New("token is required")
	}
	if strings.ContainsAny(token, "\r\n\t") {
		return "", errors.New("token must not contain whitespace control characters")
	}
	if err := config.ValidateProfileName(profile); err != nil {
		return "", err
	}

	if s.PreferKeyring {
		if err := keyring.Set(keyringService, keyringUser(profile), token); err == nil {
			_ = s.deleteFile(profile) // avoid stale file copy
			return "keyring", nil
		}
		// Keyring write failed; file becomes source of truth. Drop any older
		// keyring value so Get does not prefer a stale PAT over the new file.
		_ = keyring.Delete(keyringService, keyringUser(profile))
	}
	if err := s.setFile(profile, token); err != nil {
		return "", err
	}
	return "file", nil
}

// Get returns the PAT for profile, or empty if missing.
func (s *Store) Get(profile string) (token string, storage string, err error) {
	profile = strings.TrimSpace(profile)
	if profile == "" {
		return "", "", errors.New("profile is required")
	}

	if s.PreferKeyring {
		tok, kerr := keyring.Get(keyringService, keyringUser(profile))
		if kerr == nil && tok != "" {
			return tok, "keyring", nil
		}
		if kerr != nil && !errors.Is(kerr, keyring.ErrNotFound) {
			// fall through to file
		}
	}
	tok, ferr := s.getFile(profile)
	if ferr != nil {
		if errors.Is(ferr, os.ErrNotExist) {
			return "", "", nil
		}
		return "", "", ferr
	}
	if tok == "" {
		return "", "", nil
	}
	return tok, "file", nil
}

// Delete removes the PAT for profile from keyring and file.
func (s *Store) Delete(profile string) error {
	profile = strings.TrimSpace(profile)
	if profile == "" {
		return errors.New("profile is required")
	}
	if s.PreferKeyring {
		_ = keyring.Delete(keyringService, keyringUser(profile))
	}
	return s.deleteFile(profile)
}

type fileStore map[string]string

func (s *Store) filePath() (string, error) {
	dir, err := s.dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, fileName), nil
}

func (s *Store) loadFile() (fileStore, error) {
	path, err := s.filePath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fileStore{}, nil
		}
		return nil, err
	}
	out := fileStore{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			continue
		}
		out[parts[0]] = parts[1]
	}
	return out, nil
}

func (s *Store) saveFile(m fileStore) error {
	dir, err := s.dir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	path, err := s.filePath()
	if err != nil {
		return err
	}
	var b strings.Builder
	b.WriteString("# rstar credentials — mode 0600; prefer OS keyring when available\n")
	// Stable order keeps diffs quiet if a human opens the file.
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(&b, "%s\t%s\n", k, m[k])
	}
	return writeFileAtomic(path, []byte(b.String()), 0o600)
}

func writeFileAtomic(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	ok := false
	defer func() {
		if !ok {
			_ = os.Remove(tmpName)
		}
	}()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	ok = true
	return nil
}

func (s *Store) setFile(profile, token string) error {
	m, err := s.loadFile()
	if err != nil {
		return err
	}
	m[profile] = token
	return s.saveFile(m)
}

func (s *Store) getFile(profile string) (string, error) {
	m, err := s.loadFile()
	if err != nil {
		return "", err
	}
	tok, ok := m[profile]
	if !ok {
		return "", os.ErrNotExist
	}
	return tok, nil
}

func (s *Store) deleteFile(profile string) error {
	m, err := s.loadFile()
	if err != nil {
		return err
	}
	if _, ok := m[profile]; !ok {
		return nil
	}
	delete(m, profile)
	if len(m) == 0 {
		path, err := s.filePath()
		if err != nil {
			return err
		}
		_ = os.Remove(path)
		return nil
	}
	return s.saveFile(m)
}
