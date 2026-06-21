package tools

import (
	"context"
	"net/url"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// mockDoer is a test implementation of Doer that returns canned responses.
type mockDoer struct {
	// apiResponses maps "METHOD /path" to (status, body).
	apiResponses map[string]*Result
	rawResponses map[string]*Result
}

func newMockDoer() *mockDoer {
	return &mockDoer{
		apiResponses: map[string]*Result{
			"GET /instances":          {Status: 200, Body: []byte(`[{"id":1,"display_name":"bot-one","status":"running"}]`)},
			"POST /instances":         {Status: 201, Body: []byte(`{"id":2,"status":"created"}`)},
			"POST /instances/1/start": {Status: 200, Body: []byte(`{"status":"starting"}`)},
		},
		rawResponses: map[string]*Result{
			"GET /api/v1/instances": {Status: 200, Body: []byte(`[{"id":1,"display_name":"bot-one","status":"running"}]`)},
		},
	}
}

func (m *mockDoer) API(ctx context.Context, method, path string, query url.Values, body any) (*Result, error) {
	key := strings.ToUpper(method) + " " + path
	if r, ok := m.apiResponses[key]; ok {
		return r, nil
	}
	// Default: 404
	return &Result{Status: 404, Body: []byte(`{"error":"not found"}`)}, nil
}

func (m *mockDoer) Raw(ctx context.Context, method, path string, query url.Values, body any) (*Result, error) {
	key := strings.ToUpper(method) + " " + path
	if r, ok := m.rawResponses[key]; ok {
		return r, nil
	}
	return &Result{Status: 404, Body: []byte(`{"error":"not found"}`)}, nil
}

// connectMock wires an in-memory MCP client to a server with all tools registered
// against the provided Doer, returning the connected client session.
func connectMock(t *testing.T, d Doer) *mcp.ClientSession {
	t.Helper()

	server := mcp.NewServer(&mcp.Implementation{Name: "claworc", Version: "test"}, nil)
	Register(server, d)

	clientT, serverT := mcp.NewInMemoryTransports()
	ctx := context.Background()
	if _, err := server.Connect(ctx, serverT, nil); err != nil {
		t.Fatalf("server.Connect: %v", err)
	}
	mcpClient := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "test"}, nil)
	cs, err := mcpClient.Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatalf("client.Connect: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

func callText(t *testing.T, cs *mcp.ClientSession, name string, args map[string]any) (string, bool) {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("CallTool %s: %v", name, err)
	}
	if len(res.Content) == 0 {
		t.Fatalf("CallTool %s: empty content", name)
	}
	text := res.Content[0].(*mcp.TextContent).Text
	return text, res.IsError
}

func TestToolsRegistered(t *testing.T) {
	d := newMockDoer()
	cs := connectMock(t, d)

	var names []string
	for tool, err := range cs.Tools(context.Background(), nil) {
		if err != nil {
			t.Fatalf("listing tools: %v", err)
		}
		names = append(names, tool.Name)
	}
	if len(names) < 60 {
		t.Fatalf("expected at least 60 tools, got %d: %v", len(names), names)
	}
	// Spot-check that key tools and the escape hatch are present.
	for _, want := range []string{"claworc_list_instances", "claworc_create_instance", "claworc_request"} {
		if !contains(names, want) {
			t.Errorf("missing tool %q", want)
		}
	}
}

func TestListInstances(t *testing.T) {
	d := newMockDoer()
	cs := connectMock(t, d)

	text, isErr := callText(t, cs, "claworc_list_instances", nil)
	if isErr {
		t.Fatalf("unexpected error result: %s", text)
	}
	if !strings.Contains(text, "bot-one") || !strings.Contains(text, "HTTP 200") {
		t.Fatalf("unexpected response: %s", text)
	}
}

func TestStartInstance(t *testing.T) {
	d := newMockDoer()
	cs := connectMock(t, d)

	text, isErr := callText(t, cs, "claworc_start_instance", map[string]any{"id": 1})
	if isErr {
		t.Fatalf("unexpected error result: %s", text)
	}
	if !strings.Contains(text, "starting") {
		t.Fatalf("unexpected response: %s", text)
	}
}

func TestCreateInstanceValidation(t *testing.T) {
	d := newMockDoer()
	cs := connectMock(t, d)

	// Missing display_name should fail validation before any HTTP call.
	text, isErr := callText(t, cs, "claworc_create_instance", map[string]any{})
	if !isErr {
		t.Fatalf("expected validation error, got: %s", text)
	}
	if !strings.Contains(text, "display_name") {
		t.Fatalf("unexpected error text: %s", text)
	}
}

func TestRawRequest(t *testing.T) {
	d := newMockDoer()
	cs := connectMock(t, d)

	text, isErr := callText(t, cs, "claworc_request", map[string]any{
		"method": "GET",
		"path":   "/api/v1/instances",
	})
	if isErr {
		t.Fatalf("unexpected error result: %s", text)
	}
	if !strings.Contains(text, "bot-one") {
		t.Fatalf("unexpected response: %s", text)
	}
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}
