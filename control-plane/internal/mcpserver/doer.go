// Package mcpserver embeds the MCP server in the control plane.
//
// It provides an in-process implementation of the tools.Doer interface that
// replays the caller's Authorization header through the chi router, so the
// full middleware/authorization chain runs exactly as for external API calls.
package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"

	"github.com/gluk-w/claworc/mcp-server/tools"
)

// inProcessDoer implements tools.Doer by replaying HTTP requests through the
// chi router in-process, forwarding the captured Authorization header so that
// RequireAuth and all downstream role checks run unchanged.
type inProcessDoer struct {
	router     http.Handler
	authHeader string
}

var _ tools.Doer = (*inProcessDoer)(nil)

// API calls the control-plane endpoint at /api/v1/<path>.
func (d *inProcessDoer) API(ctx context.Context, method, path string, query url.Values, body any) (*tools.Result, error) {
	return d.do(ctx, method, "/api/v1"+path, query, body)
}

// Raw calls the control-plane endpoint at the given path verbatim.
func (d *inProcessDoer) Raw(ctx context.Context, method, path string, query url.Values, body any) (*tools.Result, error) {
	return d.do(ctx, method, path, query, body)
}

func (d *inProcessDoer) do(ctx context.Context, method, path string, query url.Values, body any) (*tools.Result, error) {
	// Build the request URL with query string.
	reqURL := path
	if len(query) > 0 {
		reqURL = path + "?" + query.Encode()
	}

	// Serialize the body.
	var bodyReader *bytes.Reader
	if body != nil {
		var data []byte
		var err error
		switch v := body.(type) {
		case json.RawMessage:
			data = v
		default:
			data, err = json.Marshal(body)
			if err != nil {
				return nil, err
			}
		}
		bodyReader = bytes.NewReader(data)
	} else {
		bodyReader = bytes.NewReader(nil)
	}

	req, err := http.NewRequestWithContext(ctx, method, reqURL, bodyReader)
	if err != nil {
		return nil, err
	}

	// Replay the captured Authorization header.
	if d.authHeader != "" {
		req.Header.Set("Authorization", d.authHeader)
	}

	// Set Content-Type when there is a body.
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	rec := httptest.NewRecorder()
	d.router.ServeHTTP(rec, req)

	return &tools.Result{
		Status: rec.Code,
		Body:   rec.Body.Bytes(),
	}, nil
}
