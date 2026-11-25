// Package mcpserver serves ADR 0001 domain gateways over MCP stdio.
//
// Dispatch goes through the in-process catalog + apiclient — never by exec'ing
// the CLI as a subprocess.
package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/turgs/rstar-cli/internal/apiclient"
	"github.com/turgs/rstar-cli/internal/catalog"
	"github.com/turgs/rstar-cli/internal/output"
	"github.com/turgs/rstar-cli/internal/version"
)

// Name is the MCP implementation name (CLI serving MCP, not a separate product).
const Name = "rstar-cli"

// Config selects the served surface.
type Config struct {
	// ReadOnly drops write actions and refuses write dispatch (default true).
	ReadOnly bool
	// Domains narrows the served domains. Empty means all catalog domains.
	// Unknown keys fail closed at startup.
	Domains []string
	// Version reported in initialize; defaults to internal/version.
	Version string
}

// Server holds the filtered catalog and dispatcher.
type Server struct {
	domains  []catalog.Domain
	dispatch *catalog.Dispatcher
	version  string
	readOnly bool
}

// New validates domain filters and builds the server.
func New(client *apiclient.Client, profile catalog.ProfileInfo, cfg Config) (*Server, error) {
	if client == nil {
		return nil, fmt.Errorf("mcpserver: API client is required")
	}
	domains, err := catalog.Filter(cfg.Domains)
	if err != nil {
		return nil, err
	}
	readOnly := cfg.ReadOnly
	if readOnly {
		domains = catalog.FilterReadOnly(domains)
		if len(domains) == 0 {
			return nil, fmt.Errorf("mcpserver: no read-only actions remain after filters")
		}
	}
	ver := cfg.Version
	if ver == "" {
		ver = version.Version
	}
	return &Server{
		domains: domains,
		dispatch: &catalog.Dispatcher{
			Client:      client,
			Profile:     profile,
			AllowWrites: !readOnly,
		},
		version:  ver,
		readOnly: readOnly,
	}, nil
}

// Domains returns the served domain list.
func (s *Server) Domains() []catalog.Domain {
	return s.domains
}

// BuildMCPServer registers one gateway tool per domain.
func (s *Server) BuildMCPServer(logger *slog.Logger) *mcp.Server {
	if logger == nil {
		logger = slog.Default()
	}
	srv := mcp.NewServer(&mcp.Implementation{Name: Name, Version: s.version}, nil)
	for _, d := range s.domains {
		dom := d // capture
		schema := dom.InputSchema()
		tool := &mcp.Tool{
			Name:        dom.Tool,
			Description: dom.Description(),
			InputSchema: schema,
		}
		srv.AddTool(tool, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return s.handleTool(ctx, logger, dom, req)
		})
	}
	return srv
}

// gatewayArgs is the {"action","params"} calling convention.
type gatewayArgs struct {
	Action string         `json:"action"`
	Params map[string]any `json:"params"`
}

func (s *Server) handleTool(ctx context.Context, logger *slog.Logger, dom catalog.Domain, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if logger == nil {
		logger = slog.Default()
	}
	var args gatewayArgs
	if req != nil && req.Params != nil && len(req.Params.Arguments) > 0 {
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return textError(fmt.Sprintf("invalid arguments: %v", err)), nil
		}
	}
	if args.Action == "" {
		return textError(`missing "action"; call describe or pass {"action":"…","params":{…}}`), nil
	}

	logger.Info("mcp tool", "tool", dom.Tool, "action", args.Action, "read_only", s.readOnly)

	res, err := s.dispatch.Call(ctx, dom.Key, args.Action, args.Params)
	if err != nil {
		ce := output.AsCLIError(err)
		payload := output.Envelope{OK: false, Error: &ce.Body, Summary: ce.Body.Message}
		b, _ := json.MarshalIndent(payload, "", "  ")
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: string(b)}},
			IsError: true,
		}, nil
	}

	env := output.Envelope{OK: true, Data: res.Data, Summary: res.Summary, Meta: res.Meta}
	b, err := json.MarshalIndent(env, "", "  ")
	if err != nil {
		return textError(err.Error()), nil
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(b)}},
	}, nil
}

func textError(msg string) *mcp.CallToolResult {
	payload := output.Envelope{
		OK:      false,
		Summary: msg,
		Error:   &output.ErrorBody{Code: output.CodeUsage, Message: msg},
	}
	b, _ := json.MarshalIndent(payload, "", "  ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(b)}},
		IsError: true,
	}
}

// RunStdio serves MCP on stdin/stdout until the client disconnects.
func RunStdio(ctx context.Context, client *apiclient.Client, profile catalog.ProfileInfo, cfg Config, logger *slog.Logger) error {
	s, err := New(client, profile, cfg)
	if err != nil {
		return err
	}
	if logger == nil {
		logger = slog.Default()
	}
	logger.Info("MCP server starting", "tools", len(s.Domains()), "read_only", s.readOnly)
	return s.BuildMCPServer(logger).Run(ctx, &mcp.StdioTransport{})
}
