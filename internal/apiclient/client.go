// Package apiclient is the shared HTTP client for /api/v1 (CLI + MCP).
package apiclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/turgs/rstar-cli/internal/config"
	"github.com/turgs/rstar-cli/internal/output"
)

// Client talks to one practice host with a Bearer PAT.
type Client struct {
	BaseURL    string
	Token      string
	HTTPClient *http.Client
	UserAgent  string
}

// New builds a client. baseURL is the practice origin (no /api/v1).
func New(baseURL, token string) *Client {
	return &Client{
		BaseURL: config.NormalizeBaseURL(baseURL),
		Token:   strings.TrimSpace(token),
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
			// Practice /api/v1 should not redirect; refuse so Bearer is never
			// replayed onto an unexpected hop (same-host different port, etc.).
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return fmt.Errorf("refusing HTTP redirect from practice API")
			},
		},
		UserAgent: "rstar-cli",
	}
}

// Response is a decoded API reply.
type Response struct {
	StatusCode int
	Header     http.Header
	// Body is the raw JSON object (may include data, errors, pagination, rate_limit).
	Body map[string]any
	Raw  []byte
}

// Data returns body["data"].
func (r *Response) Data() any {
	if r == nil || r.Body == nil {
		return nil
	}
	return r.Body["data"]
}

// Errors returns body["errors"] entries when present.
func (r *Response) Errors() []map[string]any {
	if r == nil || r.Body == nil {
		return nil
	}
	raw, ok := r.Body["errors"].([]any)
	if !ok {
		return nil
	}
	out := make([]map[string]any, 0, len(raw))
	for _, e := range raw {
		if m, ok := e.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

// Do performs method on an /api/v1-relative path (e.g. "/clients").
func (c *Client) Do(ctx context.Context, method, path string, query url.Values, body any) (*Response, error) {
	if c == nil {
		return nil, output.ErrConfig("API client is not configured")
	}
	if c.BaseURL == "" {
		return nil, output.ErrConfig("practice base URL is not set").WithHint("Pass --base-url or run 'rstar auth login --base-url https://YOUR.receptionstar.com TOKEN'.")
	}
	if err := config.ValidateBaseURL(c.BaseURL); err != nil {
		return nil, output.ErrConfig(err.Error()).WithHint("Pass --base-url https://YOUR.receptionstar.com or re-run 'rstar auth login'.")
	}
	if c.Token == "" {
		return nil, output.ErrAuth("not authenticated")
	}

	full, err := c.resolve(path, query)
	if err != nil {
		return nil, output.ErrUsage(err.Error())
	}

	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, output.NewError(output.CodeAPI, err.Error(), output.ExitAPI)
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, full, bodyReader)
	if err != nil {
		return nil, output.ErrNetwork(err)
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.UserAgent)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	res, err := httpClient.Do(req)
	if err != nil {
		return nil, output.ErrNetwork(err)
	}
	defer res.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(res.Body, 2<<20)) // 2 MiB cap
	if err != nil {
		return nil, output.ErrNetwork(err)
	}

	out := &Response{StatusCode: res.StatusCode, Header: res.Header.Clone(), Raw: raw}
	if len(raw) > 0 {
		var m map[string]any
		if err := json.Unmarshal(raw, &m); err == nil {
			out.Body = m
		}
	}

	if res.StatusCode >= 200 && res.StatusCode < 300 {
		return out, nil
	}
	return out, mapAPIError(out)
}

// Get is Do GET.
func (c *Client) Get(ctx context.Context, path string, query url.Values) (*Response, error) {
	return c.Do(ctx, http.MethodGet, path, query, nil)
}

// Post is Do POST with JSON body.
func (c *Client) Post(ctx context.Context, path string, query url.Values, body any) (*Response, error) {
	return c.Do(ctx, http.MethodPost, path, query, body)
}

// Patch is Do PATCH with JSON body.
func (c *Client) Patch(ctx context.Context, path string, query url.Values, body any) (*Response, error) {
	return c.Do(ctx, http.MethodPatch, path, query, body)
}

// Delete is Do DELETE.
func (c *Client) Delete(ctx context.Context, path string, query url.Values) (*Response, error) {
	return c.Do(ctx, http.MethodDelete, path, query, nil)
}

func (c *Client) resolve(apiPath string, query url.Values) (string, error) {
	apiPath = strings.TrimSpace(apiPath)
	if apiPath == "" {
		return "", fmt.Errorf("empty path")
	}
	if strings.Contains(apiPath, "://") {
		return "", fmt.Errorf("path must be relative to /api/v1, not an absolute URL")
	}
	apiPath = strings.TrimPrefix(apiPath, "/")
	for _, seg := range strings.Split(apiPath, "/") {
		if seg == "." || seg == ".." {
			return "", fmt.Errorf("invalid path segment %q", seg)
		}
	}
	root := config.APIRoot(c.BaseURL)
	full, err := url.JoinPath(root, apiPath)
	if err != nil {
		return "", err
	}
	u, err := url.Parse(full)
	if err != nil {
		return "", err
	}
	base, err := url.Parse(root)
	if err != nil {
		return "", err
	}
	if u.Scheme != base.Scheme || u.Host != base.Host {
		return "", fmt.Errorf("path escapes practice host")
	}
	// Clean after parse so percent-encoded %2e%2e cannot walk out of /api/v1.
	basePath := path.Clean("/" + strings.Trim(base.Path, "/"))
	if basePath == "." {
		basePath = "/"
	}
	cleaned := path.Clean(u.Path)
	if cleaned != basePath && !strings.HasPrefix(cleaned, basePath+"/") {
		return "", fmt.Errorf("path escapes API root")
	}
	u.Path = cleaned
	if query != nil {
		u.RawQuery = query.Encode()
	}
	return u.String(), nil
}

func mapAPIError(res *Response) error {
	code := output.CodeAPI
	msg := fmt.Sprintf("API request failed (%d)", res.StatusCode)
	hint := ""
	field := ""
	retryable := false

	switch res.StatusCode {
	case 401:
		code = output.CodeAuth
		msg = "unauthorized"
		hint = "Check the PAT and that API access is enabled for your user on this practice."
	case 403:
		code = output.CodeForbidden
		msg = "forbidden"
	case 404:
		code = output.CodeNotFound
		msg = "not found"
	case 422:
		code = output.CodeValidation
		msg = "validation failed"
	case 429:
		code = output.CodeRateLimit
		msg = "rate limit exceeded"
		retryable = true
		hint = "Wait and retry; check rate_limit in the response meta."
	}

	// ReceptionStar shapes (ApiAuthentication + ClientsController + merge):
	//   { "errors": [ { "field", "message", "code?", "hint?", "retryable?" } ] }
	//   { "error": "string" }   // Client::Merger::MergeError
	if errs := res.Errors(); len(errs) > 0 {
		e := errs[0]
		if m, ok := e["message"].(string); ok && m != "" {
			msg = m
		}
		if c, ok := e["code"].(string); ok && c != "" {
			code = c
		}
		if h, ok := e["hint"].(string); ok && h != "" {
			hint = h
		}
		if f, ok := e["field"].(string); ok && f != "" {
			field = f
		}
		if r, ok := e["retryable"].(bool); ok {
			retryable = r
		}
	} else if res.Body != nil {
		switch v := res.Body["error"].(type) {
		case string:
			if strings.TrimSpace(v) != "" {
				msg = strings.TrimSpace(v)
			}
		case map[string]any:
			if m, ok := v["message"].(string); ok && m != "" {
				msg = m
			}
			if c, ok := v["code"].(string); ok && c != "" {
				code = c
			}
		}
	} else if len(res.Raw) > 0 {
		msg = strings.TrimSpace(string(res.Raw))
		if len(msg) > 200 {
			msg = msg[:200] + "…"
		}
	}

	ce := output.ErrAPI(code, msg, res.StatusCode).WithRetryable(retryable)
	if hint != "" {
		ce = ce.WithHint(hint)
	}
	if field != "" {
		ce = ce.WithField(field)
	}
	return ce
}
