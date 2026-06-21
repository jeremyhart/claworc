package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gluk-w/claworc/mcp-server/internal/client"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// newMockControlPlane returns an httptest server that emulates the parts of the
// Claworc API the tools depend on: cookie login plus a couple of endpoints.
func newMockControlPlane(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	mux.HandleFunc("/api/v1/auth/login", func(w http.ResponseWriter, r *http.Request) {
		var body struct{ Username, Password string }
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.Username != "admin" || body.Password != "secret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		http.SetCookie(w, &http.Cookie{Name: "claworc_session", Value: "test-session", Path: "/"})
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":1,"name":"admin","role":"admin"}`))
	})

	requireSession := func(r *http.Request) bool {
		ck, err := r.Cookie("claworc_session")
		return err == nil && ck.Value == "test-session"
	}

	mux.HandleFunc("/api/v1/instances", func(w http.ResponseWriter, r *http.Request) {
		if !requireSession(r) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch r.Method {
		case http.MethodGet:
			_, _ = w.Write([]byte(`[{"id":1,"display_name":"bot-one","status":"running"}]`))
		case http.MethodPost:
			body, _ := json.Marshal(map[string]any{"id": 2, "status": "created"})
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write(body)
		}
	})

	mux.HandleFunc("/api/v1/instances/1/start", func(w http.ResponseWriter, r *http.Request) {
		if !requireSession(r) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(`{"status":"starting"}`))
	})

	return httptest.NewServer(mux)
}

// connect wires a real MCP client to an in-process server with all tools
// registered, returning the connected client session.
func connect(t *testing.T, baseURL string) *mcp.ClientSession {
	t.Helper()
	c, err := client.New(client.Config{BaseURL: baseURL, Username: "admin", Password: "secret"})
	if err != nil {
		t.Fatalf("client.New: %v", err)
	}

	server := mcp.NewServer(&mcp.Implementation{Name: "claworc", Version: "test"}, nil)
	Register(server, c)

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
	srv := newMockControlPlane(t)
	defer srv.Close()
	cs := connect(t, srv.URL)

	var names []string
	for tool, err := range cs.Tools(context.Background(), nil) {
		if err != nil {
			t.Fatalf("listing tools: %v", err)
		}
		names = append(names, tool.Name)
	}
	if len(names) < 30 {
		t.Fatalf("expected at least 30 tools, got %d: %v", len(names), names)
	}
	// Spot-check that a few key tools and the escape hatch are present.
	for _, want := range []string{"claworc_list_instances", "claworc_create_instance", "claworc_request"} {
		if !contains(names, want) {
			t.Errorf("missing tool %q", want)
		}
	}
}

func TestListInstances(t *testing.T) {
	srv := newMockControlPlane(t)
	defer srv.Close()
	cs := connect(t, srv.URL)

	text, isErr := callText(t, cs, "claworc_list_instances", nil)
	if isErr {
		t.Fatalf("unexpected error result: %s", text)
	}
	if !strings.Contains(text, "bot-one") || !strings.Contains(text, "HTTP 200") {
		t.Fatalf("unexpected response: %s", text)
	}
}

func TestStartInstance(t *testing.T) {
	srv := newMockControlPlane(t)
	defer srv.Close()
	cs := connect(t, srv.URL)

	text, isErr := callText(t, cs, "claworc_start_instance", map[string]any{"id": 1})
	if isErr {
		t.Fatalf("unexpected error result: %s", text)
	}
	if !strings.Contains(text, "starting") {
		t.Fatalf("unexpected response: %s", text)
	}
}

func TestCreateInstanceValidation(t *testing.T) {
	srv := newMockControlPlane(t)
	defer srv.Close()
	cs := connect(t, srv.URL)

	// Missing display_name should fail before any HTTP call — either via the
	// SDK's required-property schema validation or the handler's own guard.
	text, isErr := callText(t, cs, "claworc_create_instance", map[string]any{})
	if !isErr {
		t.Fatalf("expected validation error, got: %s", text)
	}
	if !strings.Contains(text, "display_name") {
		t.Fatalf("unexpected error text: %s", text)
	}
}

func TestRawRequest(t *testing.T) {
	srv := newMockControlPlane(t)
	defer srv.Close()
	cs := connect(t, srv.URL)

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
