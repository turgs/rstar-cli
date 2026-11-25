// Package config loads rstar profiles (practice base URL) from disk and env.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

const (
	// EnvProfile selects the active profile name.
	EnvProfile = "RSTAR_PROFILE"
	// EnvBaseURL overrides the practice API base URL (no /api/v1 suffix).
	EnvBaseURL = "RSTAR_BASE_URL"
	// EnvToken overrides the PAT (skips keyring). Prefer keyring in normal use.
	EnvToken = "RSTAR_TOKEN"
	// EnvConfigDir overrides the config directory (tests).
	EnvConfigDir = "RSTAR_CONFIG_DIR"
)

// Profile is one named practice connection.
type Profile struct {
	// Name is the profile key (e.g. "acme").
	Name string `yaml:"-" json:"name"`
	// BaseURL is the practice origin, e.g. https://acme.receptionstar.com
	BaseURL string `yaml:"base_url" json:"base_url"`
}

// File is the on-disk config shape.
type File struct {
	ActiveProfile string             `yaml:"active_profile" json:"active_profile"`
	Profiles      map[string]Profile `yaml:"profiles" json:"profiles"`
}

// Config is the resolved runtime view.
type Config struct {
	ProfileName string
	BaseURL     string
	// Token comes from env or auth store; may be empty until login.
	Token string
	// ConfigDir is where config.yaml lives.
	ConfigDir string
	// Source describes where BaseURL came from (env, profile, flag).
	BaseURLSource string
	TokenSource   string
}

var (
	testConfigDir string
	testMu        sync.Mutex
)

// SetTestConfigDir points Load at a temp dir (tests only).
func SetTestConfigDir(dir string) {
	testMu.Lock()
	defer testMu.Unlock()
	testConfigDir = dir
}

// ResetTestConfigDir clears the test override.
func ResetTestConfigDir() {
	SetTestConfigDir("")
}

// Dir returns the config directory (~/.config/rstar by default).
func Dir() (string, error) {
	testMu.Lock()
	override := testConfigDir
	testMu.Unlock()
	if override != "" {
		return override, nil
	}
	if v := strings.TrimSpace(os.Getenv(EnvConfigDir)); v != "" {
		return v, nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "rstar"), nil
}

// Path returns the config.yaml path.
func Path() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.yaml"), nil
}

// LoadFile reads config.yaml or returns an empty file.
func LoadFile() (*File, error) {
	path, err := Path()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &File{Profiles: map[string]Profile{}}, nil
		}
		return nil, err
	}
	var f File
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if f.Profiles == nil {
		f.Profiles = map[string]Profile{}
	}
	for name, p := range f.Profiles {
		p.Name = name
		f.Profiles[name] = p
	}
	return &f, nil
}

// SaveFile writes config.yaml with mode 0600.
func SaveFile(f *File) error {
	dir, err := Dir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	path, err := Path()
	if err != nil {
		return err
	}
	if f.Profiles == nil {
		f.Profiles = map[string]Profile{}
	}
	data, err := yaml.Marshal(f)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// ResolveOptions control Load.
type ResolveOptions struct {
	// ProfileName overrides active profile (flag).
	ProfileName string
	// BaseURL overrides everything (flag).
	BaseURL string
	// Token overrides everything (flag/env already applied outside).
	Token string
}

// Load resolves profile + env into a Config. Token is not loaded from keyring
// here — callers pass Token via options or auth.Store.
func Load(opts ResolveOptions) (*Config, error) {
	dir, err := Dir()
	if err != nil {
		return nil, err
	}
	file, err := LoadFile()
	if err != nil {
		return nil, err
	}

	cfg := &Config{ConfigDir: dir}

	name := strings.TrimSpace(opts.ProfileName)
	if name == "" {
		name = strings.TrimSpace(os.Getenv(EnvProfile))
	}
	if name == "" {
		name = strings.TrimSpace(file.ActiveProfile)
	}
	if name == "" && len(file.Profiles) == 1 {
		for n := range file.Profiles {
			name = n
		}
	}
	if name == "" {
		name = "default"
	}
	cfg.ProfileName = name

	if u := strings.TrimSpace(opts.BaseURL); u != "" {
		cfg.BaseURL = NormalizeBaseURL(u)
		cfg.BaseURLSource = "flag"
	} else if u := strings.TrimSpace(os.Getenv(EnvBaseURL)); u != "" {
		cfg.BaseURL = NormalizeBaseURL(u)
		cfg.BaseURLSource = "env"
	} else if p, ok := file.Profiles[name]; ok && strings.TrimSpace(p.BaseURL) != "" {
		cfg.BaseURL = NormalizeBaseURL(p.BaseURL)
		cfg.BaseURLSource = "profile"
	}

	if t := strings.TrimSpace(opts.Token); t != "" {
		cfg.Token = t
		cfg.TokenSource = "flag"
	} else if t := strings.TrimSpace(os.Getenv(EnvToken)); t != "" {
		cfg.Token = t
		cfg.TokenSource = "env"
	}

	return cfg, nil
}

// UpsertProfile writes base URL into a named profile and optionally activates it.
func UpsertProfile(name, baseURL string, activate bool) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("profile name is required")
	}
	if err := ValidateProfileName(name); err != nil {
		return err
	}
	baseURL = NormalizeBaseURL(baseURL)
	if baseURL == "" {
		return errors.New("base URL is required")
	}

	file, err := LoadFile()
	if err != nil {
		return err
	}
	file.Profiles[name] = Profile{Name: name, BaseURL: baseURL}
	if activate || file.ActiveProfile == "" {
		file.ActiveProfile = name
	}
	return SaveFile(file)
}

// SetActiveProfile switches the default profile.
func SetActiveProfile(name string) error {
	name = strings.TrimSpace(name)
	file, err := LoadFile()
	if err != nil {
		return err
	}
	if _, ok := file.Profiles[name]; !ok {
		return fmt.Errorf("unknown profile %q", name)
	}
	file.ActiveProfile = name
	return SaveFile(file)
}

// NormalizeBaseURL trims trailing slashes and a trailing /api/v1.
func NormalizeBaseURL(u string) string {
	u = strings.TrimSpace(u)
	u = strings.TrimRight(u, "/")
	u = strings.TrimSuffix(u, "/api/v1")
	u = strings.TrimRight(u, "/")
	return u
}

// ValidateProfileName allows simple profile keys.
func ValidateProfileName(name string) error {
	if name == "" {
		return errors.New("empty profile name")
	}
	for _, r := range name {
		ok := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_'
		if !ok {
			return fmt.Errorf("invalid profile name %q (use letters, digits, -, _)", name)
		}
	}
	return nil
}

// APIRoot returns baseURL + /api/v1.
func APIRoot(baseURL string) string {
	return NormalizeBaseURL(baseURL) + "/api/v1"
}
