// Package release fetches GitHub release metadata and verifies install archives.
package release

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	DefaultOwner = "turgs"
	DefaultRepo  = "rstar-cli"
	userAgent    = "rstar-cli-upgrade"
)

// Asset is one file attached to a GitHub release.
type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

// Info is the subset of a GitHub release needed for install/upgrade.
type Info struct {
	TagName string  `json:"tag_name"`
	Name    string  `json:"name"`
	Draft   bool    `json:"draft"`
	Assets  []Asset `json:"assets"`
}

// Version returns the release version without a leading "v".
func (r Info) Version() string {
	return strings.TrimPrefix(strings.TrimSpace(r.TagName), "v")
}

// AssetByName finds an asset by exact filename.
func (r Info) AssetByName(name string) (Asset, bool) {
	for _, a := range r.Assets {
		if a.Name == name {
			return a, true
		}
	}
	return Asset{}, false
}

// ArchiveName is the goreleaser archive filename for a platform.
func ArchiveName(version, goos, goarch string) string {
	version = strings.TrimPrefix(version, "v")
	ext := "tar.gz"
	if goos == "windows" {
		ext = "zip"
	}
	return fmt.Sprintf("rstar_%s_%s_%s.%s", version, goos, goarch, ext)
}

// BinaryName is the executable name inside an archive.
func BinaryName(goos string) string {
	if goos == "windows" {
		return "rstar.exe"
	}
	return "rstar"
}

// Client talks to the GitHub Releases API.
type Client struct {
	HTTP    *http.Client
	Owner   string
	Repo    string
	Token   string // optional; GITHUB_TOKEN avoids anonymous rate limits
	BaseURL string // override for tests (no trailing slash)
}

func (c *Client) owner() string {
	if c.Owner != "" {
		return c.Owner
	}
	return DefaultOwner
}

func (c *Client) repo() string {
	if c.Repo != "" {
		return c.Repo
	}
	return DefaultRepo
}

func (c *Client) base() string {
	if c.BaseURL != "" {
		return strings.TrimRight(c.BaseURL, "/")
	}
	return "https://api.github.com"
}

func (c *Client) httpClient() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	// Long enough for multi-MB archives; callers still pass a context deadline.
	return &http.Client{Timeout: 5 * time.Minute}
}

// Latest fetches the latest non-draft release.
func (c *Client) Latest(ctx context.Context) (*Info, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/releases/latest", c.base(), c.owner(), c.repo())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", userAgent)
	if tok := c.token(); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	res, err := c.httpClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github releases latest: HTTP %d: %s", res.StatusCode, strings.TrimSpace(string(body)))
	}
	var info Info
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, err
	}
	if info.Draft || info.TagName == "" {
		return nil, fmt.Errorf("no published release found for %s/%s", c.owner(), c.repo())
	}
	return &info, nil
}

func (c *Client) token() string {
	if c.Token != "" {
		return c.Token
	}
	return strings.TrimSpace(os.Getenv("GITHUB_TOKEN"))
}

// Download writes url to dest (exclusive create) and returns the SHA-256 hex of the bytes.
func (c *Client) Download(ctx context.Context, url, dest string, limit int64) (sha256hex string, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", userAgent)
	if tok := c.token(); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	res, err := c.httpClient().Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 4<<10))
		return "", fmt.Errorf("download %s: HTTP %d: %s", url, res.StatusCode, strings.TrimSpace(string(body)))
	}
	f, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = f.Close()
		if err != nil {
			_ = os.Remove(dest)
		}
	}()
	return copySHA256(f, io.LimitReader(res.Body, limit+1), limit)
}

// FetchBytes downloads a small text asset (checksums.txt).
func (c *Client) FetchBytes(ctx context.Context, url string, limit int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	if tok := c.token(); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	res, err := c.httpClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 4<<10))
		return nil, fmt.Errorf("fetch %s: HTTP %d: %s", url, res.StatusCode, strings.TrimSpace(string(body)))
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("response larger than %d bytes", limit)
	}
	return data, nil
}

// TagURL is the human release page for a version.
func TagURL(owner, repo, version string) string {
	if owner == "" {
		owner = DefaultOwner
	}
	if repo == "" {
		repo = DefaultRepo
	}
	version = strings.TrimPrefix(version, "v")
	return fmt.Sprintf("https://github.com/%s/%s/releases/tag/v%s", owner, repo, version)
}
