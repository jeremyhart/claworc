package tools

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"

	"github.com/gluk-w/claworc/mcp-server/internal/client"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type rawRequestInput struct {
	Method string            `json:"method" jsonschema:"HTTP method: GET, POST, PUT, PATCH, or DELETE"`
	Path   string            `json:"path" jsonschema:"request path, e.g. /api/v1/instances or /api/v1/instances/3/start"`
	Query  map[string]string `json:"query,omitempty" jsonschema:"optional query-string parameters"`
	Body   json.RawMessage   `json:"body,omitempty" jsonschema:"optional JSON request body"`
}

// registerRaw exposes a generic request tool that can reach any control-plane
// endpoint. It is the escape hatch for operations without a dedicated typed
// tool, ensuring the full API surface remains controllable by an LLM.
func registerRaw(s *mcp.Server, c *client.Client) {
	mcp.AddTool(s, &mcp.Tool{
		Name: "claworc_request",
		Description: "Make an arbitrary authenticated request to the Claworc control-plane API. " +
			"Use this for any endpoint not covered by a dedicated tool. " +
			"Paths usually start with /api/v1. The session cookie is attached automatically.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in rawRequestInput) (*mcp.CallToolResult, any, error) {
		method := strings.ToUpper(strings.TrimSpace(in.Method))
		if method == "" {
			method = "GET"
		}
		if in.Path == "" {
			return errorResult("path is required"), nil, nil
		}

		var q url.Values
		if len(in.Query) > 0 {
			q = url.Values{}
			for k, v := range in.Query {
				q.Set(k, v)
			}
		}

		var body any
		if len(in.Body) > 0 {
			body = in.Body
		}

		resp, err := c.Raw(ctx, method, in.Path, q, body)
		if err != nil {
			return errorResult(err.Error()), nil, nil
		}
		return apiResult(resp, false), nil, nil
	})
}
