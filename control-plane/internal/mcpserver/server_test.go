package mcpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gluk-w/claworc/control-plane/internal/auth"
	"github.com/gluk-w/claworc/control-plane/internal/database"
	"github.com/gluk-w/claworc/control-plane/internal/middleware"
	"github.com/gluk-w/claworc/mcp-server/tools"
	"github.com/go-chi/chi/v5"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// setupTestDB opens an isolated in-memory SQLite database and migrates the
// tables needed for auth-related tests.
func setupTestDB(t *testing.T) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open in-memory db: %v", err)
	}
	if err := db.AutoMigrate(&database.User{}, &database.UserInstance{}, &database.APIToken{}); err != nil {
		t.Fatalf("auto-migrate: %v", err)
	}
	database.DB = db
	t.Cleanup(func() { database.DB = nil })
}

// --- Test A: Doer replays the Authorization header ---

// TestDoerForwardsAuthHeader verifies that the in-process Doer attaches the
// captured Authorization header to every replayed request. A minimal chi router
// with a ping endpoint that checks the header is used as the backend.
func TestDoerForwardsAuthHeader(t *testing.T) {
	r := chi.NewRouter()
	r.Get("/api/v1/ping", func(w http.ResponseWriter, req *http.Request) {
		if req.Header.Get("Authorization") == "Bearer good" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"ok":true}`)) //nolint:errcheck
		} else {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":"unauthorized"}`)) //nolint:errcheck
		}
	})

	t.Run("valid header forwarded", func(t *testing.T) {
		d := &inProcessDoer{router: r, authHeader: "Bearer good"}
		res, err := d.API(context.Background(), "GET", "/ping", nil, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Status != http.StatusOK {
			t.Errorf("status = %d, want 200 (body: %s)", res.Status, res.Body)
		}
	})

	t.Run("wrong header returns 401", func(t *testing.T) {
		d := &inProcessDoer{router: r, authHeader: "Bearer bad"}
		res, err := d.API(context.Background(), "GET", "/ping", nil, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Status != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401 (body: %s)", res.Status, res.Body)
		}
	})

	t.Run("no header returns 401", func(t *testing.T) {
		d := &inProcessDoer{router: r, authHeader: ""}
		res, err := d.API(context.Background(), "GET", "/ping", nil, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Status != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401 (body: %s)", res.Status, res.Body)
		}
	})
}

// TestDoerRawPath verifies that Raw() uses the path verbatim (no /api/v1 prefix).
func TestDoerRawPath(t *testing.T) {
	r := chi.NewRouter()
	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`)) //nolint:errcheck
	})

	d := &inProcessDoer{router: r, authHeader: ""}
	res, err := d.Raw(context.Background(), "GET", "/health", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Status != http.StatusOK {
		t.Errorf("status = %d, want 200", res.Status)
	}
	if !strings.Contains(string(res.Body), "ok") {
		t.Errorf("body = %s, want body containing 'ok'", res.Body)
	}
}

// TestDoerBody verifies JSON body serialization and Content-Type header.
func TestDoerBody(t *testing.T) {
	r := chi.NewRouter()
	r.Post("/api/v1/echo", func(w http.ResponseWriter, req *http.Request) {
		if req.Header.Get("Content-Type") != "application/json" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		var payload map[string]string
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(payload) //nolint:errcheck
	})

	d := &inProcessDoer{router: r, authHeader: ""}
	res, err := d.API(context.Background(), "POST", "/echo", nil, map[string]string{"key": "value"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Status != http.StatusOK {
		t.Errorf("status = %d, want 200 (body: %s)", res.Status, res.Body)
	}
	if !strings.Contains(string(res.Body), "value") {
		t.Errorf("body = %s, want body containing 'value'", res.Body)
	}
}

// --- Test B: Role enforcement via tools.Register + in-process Doer ---

// makeAuthRouter builds a chi router with RequireAuth + RequireAdmin on /api/v1/admin-only
// and a plain RequireAuth route at /api/v1/instances, backed by the provided session store.
func makeAuthRouter(t *testing.T, store *auth.SessionStore) *chi.Mux {
	t.Helper()
	r := chi.NewRouter()
	r.Route("/api/v1", func(r chi.Router) {
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireAuth(store))
			// Simulates any authenticated endpoint.
			r.Get("/instances", func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`[]`)) //nolint:errcheck
			})
			r.Group(func(r chi.Router) {
				r.Use(middleware.RequireAdmin)
				r.Get("/settings", func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusOK)
					w.Write([]byte(`{"admin":true}`)) //nolint:errcheck
				})
			})
		})
	})
	return r
}

// connectWithToken wires an in-memory MCP client/server pair using the given
// Doer, and returns the connected client session.
func connectWithToken(t *testing.T, router http.Handler, authHeader string) (*mcp.ClientSession, func()) {
	t.Helper()

	doer := &inProcessDoer{router: router, authHeader: authHeader}
	server := mcp.NewServer(&mcp.Implementation{Name: "claworc", Version: "test"}, nil)
	tools.Register(server, doer)

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
	return cs, func() { _ = cs.Close() }
}

// mintBearerToken creates a user in the test DB and returns a valid bearer
// token string for that user.
func mintBearerToken(t *testing.T, username, role string) string {
	t.Helper()
	if err := database.CreateUser(&database.User{
		Username:     username,
		PasswordHash: "x",
		Role:         role,
	}); err != nil {
		t.Fatalf("create user %s: %v", username, err)
	}
	user, err := database.GetUserByUsername(username)
	if err != nil {
		t.Fatalf("get user %s: %v", username, err)
	}
	plain, hash, prefix, err := auth.GenerateAPIToken()
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	tok := &database.APIToken{
		UserID:    user.ID,
		Name:      "test",
		TokenHash: hash,
		Prefix:    prefix,
	}
	if err := database.CreateAPIToken(tok); err != nil {
		t.Fatalf("create api token: %v", err)
	}
	return plain
}

func callTool(t *testing.T, cs *mcp.ClientSession, name string, args map[string]any) (string, bool) {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("CallTool %s: %v", name, err)
	}
	if len(res.Content) == 0 {
		return "", res.IsError
	}
	text := res.Content[0].(*mcp.TextContent).Text
	return text, res.IsError
}

// TestAdminCanCallListInstances verifies that an admin bearer token
// successfully round-trips through the in-process Doer + tools.Register
// and reaches the authenticated /api/v1/instances endpoint.
func TestAdminCanCallListInstances(t *testing.T) {
	setupTestDB(t)
	store := auth.NewSessionStore()
	router := makeAuthRouter(t, store)

	plain := mintBearerToken(t, "admin-user", "admin")
	cs, cleanup := connectWithToken(t, router, "Bearer "+plain)
	defer cleanup()

	text, isErr := callTool(t, cs, "claworc_list_instances", nil)
	if isErr {
		t.Errorf("expected success, got error result: %s", text)
	}
	if !strings.Contains(text, "HTTP 200") {
		t.Errorf("expected HTTP 200 in result, got: %s", text)
	}
}

// TestAdminOnlyRouteEnforced verifies that a non-admin user receives a 403
// when calling a tool that hits an admin-only route.
// The claworc_request tool (escape-hatch) is used to target /api/v1/settings.
func TestAdminOnlyRouteEnforced(t *testing.T) {
	setupTestDB(t)
	store := auth.NewSessionStore()
	router := makeAuthRouter(t, store)

	// Mint a non-admin user.
	regularPlain := mintBearerToken(t, "regular-user", "user")
	csRegular, cleanupRegular := connectWithToken(t, router, "Bearer "+regularPlain)
	defer cleanupRegular()

	// Use the escape-hatch tool to hit the admin-only settings endpoint.
	text, _ := callTool(t, csRegular, "claworc_request", map[string]any{
		"method": "GET",
		"path":   "/api/v1/settings",
	})
	// Expect HTTP 403 in the response text.
	if !strings.Contains(text, "HTTP 403") {
		t.Errorf("expected HTTP 403 for non-admin on admin route, got: %s", text)
	}
}

// TestAdminCanReachAdminOnlyRoute verifies the positive path: admin user can
// hit the admin-only settings endpoint through the in-process Doer.
func TestAdminCanReachAdminOnlyRoute(t *testing.T) {
	setupTestDB(t)
	store := auth.NewSessionStore()
	router := makeAuthRouter(t, store)

	adminPlain := mintBearerToken(t, "another-admin", "admin")
	csAdmin, cleanupAdmin := connectWithToken(t, router, "Bearer "+adminPlain)
	defer cleanupAdmin()

	text, isErr := callTool(t, csAdmin, "claworc_request", map[string]any{
		"method": "GET",
		"path":   "/api/v1/settings",
	})
	if isErr {
		t.Errorf("expected success for admin on admin route, got error: %s", text)
	}
	if !strings.Contains(text, "HTTP 200") {
		t.Errorf("expected HTTP 200 for admin on admin route, got: %s", text)
	}
}

// TestUnauthenticatedRequestBlocked verifies that a call with no bearer token
// gets 401 from the router (the outer /mcp handler already guards with
// RequireAuth, but the Doer re-runs auth on every replayed call as well).
func TestUnauthenticatedRequestBlocked(t *testing.T) {
	setupTestDB(t)
	store := auth.NewSessionStore()
	router := makeAuthRouter(t, store)

	// No auth header — all replayed requests should get 401.
	cs, cleanup := connectWithToken(t, router, "")
	defer cleanup()

	text, _ := callTool(t, cs, "claworc_list_instances", nil)
	if !strings.Contains(text, "HTTP 401") {
		t.Errorf("expected HTTP 401 for unauthenticated call, got: %s", text)
	}
}

// TestNewHandlerServes verifies that NewHandler builds an http.Handler that
// responds to a minimal HTTP request (the MCP handshake returns a valid HTTP
// response).
func TestNewHandlerServes(t *testing.T) {
	r := chi.NewRouter()
	r.Get("/api/v1/instances", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("[]")) //nolint:errcheck
	})

	h := NewHandler(r)
	req := httptest.NewRequest("POST", "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"0.1"}}}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	// The handler should return a valid HTTP response (not a 5xx).
	if w.Code >= 500 {
		t.Errorf("NewHandler returned server error: %d %s", w.Code, w.Body.String())
	}
}
