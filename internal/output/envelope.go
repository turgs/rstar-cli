// Package output defines the CLI JSON envelope shared by leaf commands and MCP.
package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// Envelope is the success shape agents parse.
//
//	{ "ok": true, "data": …, "summary": "…", "breadcrumbs": […] }
type Envelope struct {
	OK          bool           `json:"ok"`
	Data        any            `json:"data,omitempty"`
	Summary     string         `json:"summary,omitempty"`
	Breadcrumbs []Breadcrumb   `json:"breadcrumbs,omitempty"`
	Error       *ErrorBody     `json:"error,omitempty"`
	Meta        map[string]any `json:"meta,omitempty"`
}

// Breadcrumb suggests a next CLI/MCP step with optional pre-filled values.
type Breadcrumb struct {
	Label   string         `json:"label"`
	Command string         `json:"command,omitempty"`
	Hint    string         `json:"hint,omitempty"`
	Params  map[string]any `json:"params,omitempty"`
}

// ErrorBody is the failure shape agents branch on.
type ErrorBody struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	Retryable  bool   `json:"retryable"`
	Hint       string `json:"hint,omitempty"`
	Field      string `json:"field,omitempty"`
	HTTPStatus int    `json:"http_status,omitempty"`
}

// Exit codes (aligned with common CLI agent rubrics).
const (
	ExitOK        = 0
	ExitUsage     = 2
	ExitAuth      = 3
	ExitForbidden = 4
	ExitNotFound  = 5
	ExitRateLimit = 6
	ExitNetwork   = 7
	ExitAPI       = 8
)

// Codes for ErrorBody.Code.
const (
	CodeUsage         = "usage"
	CodeAuth          = "unauthorized"
	CodeForbidden     = "forbidden"
	CodeNotFound      = "not_found"
	CodeRateLimit     = "rate_limit_exceeded"
	CodeNetwork       = "network_error"
	CodeAPI           = "api_error"
	CodeValidation    = "validation_failed"
	CodeConfig        = "config_error"
	CodeNotImpl       = "not_implemented"
	CodeWriteDenied   = "write_denied"
	CodeDomainDenied  = "domain_denied"
	CodeUnknownAction = "unknown_action"
)

// CLIError is a typed error that maps to an envelope + exit code.
type CLIError struct {
	Body     ErrorBody
	ExitCode int
}

func (e *CLIError) Error() string {
	if e.Body.Hint != "" {
		return fmt.Sprintf("%s: %s (%s)", e.Body.Code, e.Body.Message, e.Body.Hint)
	}
	return fmt.Sprintf("%s: %s", e.Body.Code, e.Body.Message)
}

// NewError builds a CLIError.
func NewError(code, message string, exit int) *CLIError {
	return &CLIError{
		Body:     ErrorBody{Code: code, Message: message},
		ExitCode: exit,
	}
}

// WithHint adds a safe next-step hint.
func (e *CLIError) WithHint(hint string) *CLIError {
	e.Body.Hint = hint
	return e
}

// WithRetryable marks whether a later identical call may succeed.
func (e *CLIError) WithRetryable(v bool) *CLIError {
	e.Body.Retryable = v
	return e
}

// WithHTTP records the transport status when known.
func (e *CLIError) WithHTTP(status int) *CLIError {
	e.Body.HTTPStatus = status
	return e
}

// WithField sets the field name for validation-style errors.
func (e *CLIError) WithField(field string) *CLIError {
	e.Body.Field = field
	return e
}

// Helpers for common cases.
func ErrUsage(msg string) *CLIError {
	return NewError(CodeUsage, msg, ExitUsage).WithHint("Run 'rstar help --agent' or 'rstar commands --json'.")
}

func ErrAuth(msg string) *CLIError {
	return NewError(CodeAuth, msg, ExitAuth).WithHint("Run 'rstar auth login --base-url https://YOUR.receptionstar.com TOKEN' (Settings > Your Account > API Keys).")
}

func ErrConfig(msg string) *CLIError {
	return NewError(CodeConfig, msg, ExitUsage).WithHint("Run 'rstar doctor' and 'rstar auth status'.")
}

func ErrNetwork(err error) *CLIError {
	return NewError(CodeNetwork, err.Error(), ExitNetwork).WithRetryable(true).WithHint("Check base URL, DNS, and network; then retry.")
}

func ErrAPI(code, msg string, status int) *CLIError {
	exit := ExitAPI
	retryable := false
	switch code {
	case CodeAuth:
		exit = ExitAuth
	case CodeForbidden:
		exit = ExitForbidden
	case CodeNotFound:
		exit = ExitNotFound
	case CodeRateLimit:
		exit = ExitRateLimit
		retryable = true
	}
	return NewError(code, msg, exit).WithHTTP(status).WithRetryable(retryable)
}

// WriteSuccess writes a success envelope as JSON.
func WriteSuccess(w io.Writer, data any, summary string, crumbs ...Breadcrumb) error {
	env := Envelope{OK: true, Data: data, Summary: summary, Breadcrumbs: crumbs}
	return writeJSON(w, env)
}

// WriteError writes a failure envelope as JSON.
func WriteError(w io.Writer, err *CLIError) error {
	if err == nil {
		err = NewError(CodeAPI, "unknown error", ExitAPI)
	}
	env := Envelope{OK: false, Error: &err.Body, Summary: err.Body.Message}
	return writeJSON(w, env)
}

// WriteHuman writes plain text for interactive terminals.
func WriteHuman(w io.Writer, format string, args ...any) {
	fmt.Fprintf(w, format, args...)
	if !endsWithNewline(format) {
		fmt.Fprintln(w)
	}
}

func endsWithNewline(s string) bool {
	return len(s) > 0 && s[len(s)-1] == '\n'
}

func writeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}

// AsCLIError unwraps or wraps an error.
func AsCLIError(err error) *CLIError {
	if err == nil {
		return nil
	}
	if ce, ok := err.(*CLIError); ok {
		return ce
	}
	return NewError(CodeAPI, err.Error(), ExitAPI)
}

// ExitCodeFor returns the process exit code for err.
func ExitCodeFor(err error) int {
	if err == nil {
		return ExitOK
	}
	if se, ok := err.(*SilentExit); ok {
		return se.Code
	}
	return AsCLIError(err).ExitCode
}

// SilentExit means the command already wrote output; only the exit code matters.
type SilentExit struct {
	Code int
}

func (e *SilentExit) Error() string { return fmt.Sprintf("exit %d", e.Code) }

// AlreadyPrinted reports whether Execute should skip printing the error.
func AlreadyPrinted(err error) bool {
	_, ok := err.(*SilentExit)
	return ok
}

// DefaultOut is stdout; tests may replace.
var DefaultOut io.Writer = os.Stdout

// DefaultErr is stderr; tests may replace.
var DefaultErr io.Writer = os.Stderr

// WriteRawJSON writes any value as indented JSON (human mode data dumps).
func WriteRawJSON(w io.Writer, v any) error {
	return writeJSON(w, v)
}
