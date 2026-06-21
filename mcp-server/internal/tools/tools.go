// Package tools registers the Claworc management tools on an MCP server.
//
// Each tool maps to one or more control-plane REST endpoints. Handlers are
// intentionally thin: they translate typed input into an HTTP call and return
// the (pretty-printed) JSON response. A generic claworc_request tool provides
// full coverage of any endpoint not given a dedicated, typed wrapper.
package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/gluk-w/claworc/mcp-server/internal/client"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// apiCall describes a single REST request derived from a tool's input.
type apiCall struct {
	Method string
	Path   string // relative to /api/v1, e.g. "/instances/3/start"
	Query  url.Values
	Body   any  // JSON-encoded when non-nil
	SSE    bool // parse the response body as Server-Sent Events (logs)
}

// Register wires up every Claworc tool on the server.
func Register(s *mcp.Server, c *client.Client) {
	registerInstances(s, c)
	registerProviders(s, c)
	registerSettings(s, c)
	registerUsers(s, c)
	registerTeams(s, c)
	registerSkills(s, c)
	registerBackups(s, c)
	registerKanban(s, c)
	registerTasks(s, c)
	registerSystem(s, c)
	registerRaw(s, c)
}

// addTool registers a typed tool whose handler builds an apiCall against
// /api/v1. The build function may return an error to short-circuit with a
// validation failure before any HTTP request is made.
func addTool[In any](s *mcp.Server, c *client.Client, t *mcp.Tool, build func(In) (apiCall, error)) {
	mcp.AddTool(s, t, func(ctx context.Context, _ *mcp.CallToolRequest, in In) (*mcp.CallToolResult, any, error) {
		call, err := build(in)
		if err != nil {
			return errorResult(err.Error()), nil, nil
		}
		resp, err := c.API(ctx, call.Method, call.Path, call.Query, call.Body)
		if err != nil {
			return errorResult(err.Error()), nil, nil
		}
		return apiResult(resp, call.SSE), nil, nil
	})
}

func apiResult(resp *client.Response, sse bool) *mcp.CallToolResult {
	var text string
	switch {
	case sse:
		text = client.ParseSSE(resp.Body)
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
