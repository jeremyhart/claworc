// Package tools registers the Claworc management tools on an MCP server.
//
// Each tool maps to one or more control-plane REST endpoints. Handlers are
// intentionally thin: they translate typed input into an HTTP call and return
// the (pretty-printed) JSON response. A generic claworc_request tool provides
// full coverage of any endpoint not given a dedicated, typed wrapper.
//
// Tools depend only on the Doer interface — not on any concrete HTTP client —
// so the same tool set can be reused both by the standalone stdio binary (with
// an HTTP-client Doer) and by the embedded control-plane server (with an
// in-process Doer).
package tools

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Result is the normalised response from a Doer call.
type Result struct {
	Status int
	Body   []byte
}

// Doer executes an API call and returns the raw response. Authentication is
// the Doer's responsibility (bearer header for HTTP; replayed request context
// in-process).
type Doer interface {
	// API targets the control plane under /api/v1.
	API(ctx context.Context, method, path string, query url.Values, body any) (*Result, error)
	// Raw targets an arbitrary path (escape-hatch tool).
	Raw(ctx context.Context, method, path string, query url.Values, body any) (*Result, error)
}

// apiCall describes a single REST request derived from a tool's input.
type apiCall struct {
	Method string
	Path   string // relative to /api/v1, e.g. "/instances/3/start"
	Query  url.Values
	Body   any  // JSON-encoded when non-nil
	SSE    bool // parse the response body as Server-Sent Events (logs)
}

// Register wires up every Claworc tool on the server.
func Register(s *mcp.Server, d Doer) {
	registerInstances(s, d)
	registerProviders(s, d)
	registerSettings(s, d)
	registerUsers(s, d)
	registerTeams(s, d)
	registerSkills(s, d)
	registerBackups(s, d)
	registerKanban(s, d)
	registerTasks(s, d)
	registerSystem(s, d)
	registerRaw(s, d)
}

// addTool registers a typed tool whose handler builds an apiCall against
// /api/v1. The build function may return an error to short-circuit with a
// validation failure before any HTTP request is made.
func addTool[In any](s *mcp.Server, d Doer, t *mcp.Tool, build func(In) (apiCall, error)) {
	mcp.AddTool(s, t, func(ctx context.Context, _ *mcp.CallToolRequest, in In) (*mcp.CallToolResult, any, error) {
		call, err := build(in)
		if err != nil {
			return errorResult(err.Error()), nil, nil
		}
		resp, err := d.API(ctx, call.Method, call.Path, call.Query, call.Body)
		if err != nil {
			return errorResult(err.Error()), nil, nil
		}
		return formatResult(resp, call.SSE), nil, nil
	})
}

// formatResult formats a *Result into a CallToolResult, pretty-printing JSON
// and parsing SSE payloads when requested. IsError is set when Status >= 400.
func formatResult(resp *Result, sse bool) *mcp.CallToolResult {
	var text string
	switch {
	case sse:
		text = ParseSSE(resp.Body)
	case len(resp.Body) == 0:
		text = "(empty response)"
	default:
		text = prettyJSON(resp.Body)
	}

	result := &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: fmt.Sprintf("HTTP %d\n\n%s", resp.Status, text)},
		},
	}
	if resp.Status >= 400 {
		result.IsError = true
	}
	return result
}

func errorResult(msg string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{&mcp.TextContent{Text: "Error: " + msg}},
	}
}

func prettyJSON(raw []byte) string {
	var buf bytes.Buffer
	if err := json.Indent(&buf, raw, "", "  "); err != nil {
		return string(raw) // not JSON (e.g. plain text error) — return as-is
	}
	return buf.String()
}

// idPath builds a path like "/instances/3/start" from a uint id.
func idPath(prefix string, id uint, suffix string) string {
	return fmt.Sprintf("%s/%d%s", prefix, id, suffix)
}

// ParseSSE extracts the payloads of "data:" lines from a Server-Sent Events
// body, joining them with newlines. The control plane streams instance logs
// this way; with follow=false the stream terminates after the requested tail.
func ParseSSE(body []byte) string {
	var b strings.Builder
	scanner := bufio.NewScanner(bytes.NewReader(body))
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data:") {
			if b.Len() > 0 {
				b.WriteByte('\n')
			}
			b.WriteString(strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "))
		}
	}
	return b.String()
}
