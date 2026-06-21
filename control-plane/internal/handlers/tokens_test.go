package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gluk-w/claworc/control-plane/internal/database"
	"github.com/gluk-w/claworc/control-plane/internal/middleware"
)

// setupTokensTestDB extends the basic test DB to include the APIToken table.
func setupTokensTestDB(t *testing.T) {
	t.Helper()
	setupTestDB(t)
	if err := database.DB.AutoMigrate(&database.APIToken{}); err != nil {
		t.Fatalf("migrate APIToken: %v", err)
	}
}

// --- ListAPITokens ---

func TestListAPITokens_Empty(t *testing.T) {
	setupTokensTestDB(t)
	user := createUserWithPassword(t, "alice", "p", "admin")

	req := httptest.NewRequest("GET", "/api/v1/auth/tokens", nil)
	req = req.WithContext(middleware.WithUser(req.Context(), user))
	w := httptest.NewRecorder()
	ListAPITokens(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body: %s", w.Code, w.Body.String())
	}
	var result []interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("len = %d, want 0", len(result))
	}
}

func TestListAPITokens_HidesSecret(t *testing.T) {
	setupTokensTestDB(t)
	user := createUserWithPassword(t, "alice", "p", "admin")

	// Create a token via the handler
	createReq := httptest.NewRequest("POST", "/api/v1/auth/tokens",
		strings.NewReader(`{"name":"ci-token"}`))
	createReq.Header.Set("Content-Type", "application/json")
	createReq = createReq.WithContext(middleware.WithUser(createReq.Context(), user))
	createW := httptest.NewRecorder()
	CreateAPIToken(createW, createReq)
	if createW.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want 201, body: %s", createW.Code, createW.Body.String())
	}

	// Now list — the secret must not appear
	listReq := httptest.NewRequest("GET", "/api/v1/auth/tokens", nil)
	listReq = listReq.WithContext(middleware.WithUser(listReq.Context(), user))
	listW := httptest.NewRecorder()
	ListAPITokens(listW, listReq)

	if listW.Code != http.StatusOK {
		t.Fatalf("list status = %d, want 200", listW.Code)
	}

	// Decode the create response to get the plaintext token
	var createBody map[string]interface{}
	json.Unmarshal(createW.Body.Bytes(), &createBody)
	plaintextToken := createBody["token"].(string)

	raw := listW.Body.String()
	// The full plaintext (which is 76 chars) must not appear in the list response.
	// The prefix (e.g. "claworc_pat_d89859") is allowed — that is the display identifier.
	if strings.Contains(raw, plaintextToken) {
		t.Errorf("list response contains the plaintext token: %s", raw)
	}

	var items []map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("len = %d, want 1", len(items))
	}
	if items[0]["name"] != "ci-token" {
		t.Errorf("name = %v, want ci-token", items[0]["name"])
	}
	if _, ok := items[0]["token"]; ok {
		t.Error("token field must not appear in list response")
	}
	if _, ok := items[0]["token_hash"]; ok {
		t.Error("token_hash field must not appear in list response")
	}
}

// --- CreateAPIToken ---

func TestCreateAPIToken_ReturnsPlaintextOnce(t *testing.T) {
	setupTokensTestDB(t)
	user := createUserWithPassword(t, "alice", "p", "admin")

	req := httptest.NewRequest("POST", "/api/v1/auth/tokens",
		strings.NewReader(`{"name":"my-token"}`))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(middleware.WithUser(req.Context(), user))
	w := httptest.NewRecorder()
	CreateAPIToken(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201, body: %s", w.Code, w.Body.String())
	}
	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	token, ok := body["token"].(string)
	if !ok || token == "" {
		t.Fatal("token field missing or empty in create response")
	}
	if !strings.HasPrefix(token, "claworc_pat_") {
		t.Errorf("token = %q, want claworc_pat_... prefix", token)
	}
	prefix, ok := body["prefix"].(string)
	if !ok || prefix == "" {
		t.Fatal("prefix missing in create response")
	}
	if !strings.HasPrefix(prefix, "claworc_pat_") {
		t.Errorf("prefix = %q, want claworc_pat_... prefix", prefix)
	}
	if body["name"] != "my-token" {
		t.Errorf("name = %v, want my-token", body["name"])
	}
}

func TestCreateAPIToken_EmptyName(t *testing.T) {
	setupTokensTestDB(t)
	user := createUserWithPassword(t, "alice", "p", "admin")

	req := httptest.NewRequest("POST", "/api/v1/auth/tokens",
		strings.NewReader(`{"name":""}`))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(middleware.WithUser(req.Context(), user))
	w := httptest.NewRecorder()
	CreateAPIToken(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestCreateAPIToken_WithExpiry(t *testing.T) {
	setupTokensTestDB(t)
	user := createUserWithPassword(t, "alice", "p", "admin")

	req := httptest.NewRequest("POST", "/api/v1/auth/tokens",
		strings.NewReader(`{"name":"expiring","expires_in_days":30}`))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(middleware.WithUser(req.Context(), user))
	w := httptest.NewRecorder()
	CreateAPIToken(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201, body: %s", w.Code, w.Body.String())
	}
	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)
	if body["expires_at"] == nil {
		t.Error("expires_at should be set when expires_in_days > 0")
	}
}

// --- DeleteAPIToken ---

func TestDeleteAPIToken_Success(t *testing.T) {
	setupTokensTestDB(t)
	user := createUserWithPassword(t, "alice", "p", "admin")

	// Create a token
	createReq := httptest.NewRequest("POST", "/api/v1/auth/tokens",
		strings.NewReader(`{"name":"to-delete"}`))
	createReq.Header.Set("Content-Type", "application/json")
	createReq = createReq.WithContext(middleware.WithUser(createReq.Context(), user))
	createW := httptest.NewRecorder()
	CreateAPIToken(createW, createReq)
	if createW.Code != http.StatusCreated {
		t.Fatalf("create status = %d", createW.Code)
	}
	var created map[string]interface{}
	json.Unmarshal(createW.Body.Bytes(), &created)
	tokenID := fmt.Sprintf("%d", int(created["id"].(float64)))

	// Delete it
	delReq := httptest.NewRequest("DELETE", "/api/v1/auth/tokens/"+tokenID, nil)
	delReq = withChiAndUser(delReq, user, map[string]string{"id": tokenID})
	delW := httptest.NewRecorder()
	DeleteAPIToken(delW, delReq)

	if delW.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204, body: %s", delW.Code, delW.Body.String())
	}

	// Verify it's gone from the list
	listReq := httptest.NewRequest("GET", "/api/v1/auth/tokens", nil)
	listReq = listReq.WithContext(middleware.WithUser(listReq.Context(), user))
	listW := httptest.NewRecorder()
	ListAPITokens(listW, listReq)
	var items []interface{}
	json.Unmarshal(listW.Body.Bytes(), &items)
	if len(items) != 0 {
		t.Errorf("after delete, list len = %d, want 0", len(items))
	}
}

func TestDeleteAPIToken_CrossUserIsolation(t *testing.T) {
	setupTokensTestDB(t)
	alice := createUserWithPassword(t, "alice", "p", "admin")
	bob := createUserWithPassword(t, "bob", "p", "user")

	// Alice creates a token
	createReq := httptest.NewRequest("POST", "/api/v1/auth/tokens",
		strings.NewReader(`{"name":"alice-token"}`))
	createReq.Header.Set("Content-Type", "application/json")
	createReq = createReq.WithContext(middleware.WithUser(createReq.Context(), alice))
	createW := httptest.NewRecorder()
	CreateAPIToken(createW, createReq)
	if createW.Code != http.StatusCreated {
		t.Fatalf("create status = %d", createW.Code)
	}
	var created map[string]interface{}
	json.Unmarshal(createW.Body.Bytes(), &created)
	tokenID := fmt.Sprintf("%d", int(created["id"].(float64)))

	// Bob tries to delete Alice's token — GORM deletes 0 rows (different user_id).
	delReq := httptest.NewRequest("DELETE", "/api/v1/auth/tokens/"+tokenID, nil)
	delReq = withChiAndUser(delReq, bob, map[string]string{"id": tokenID})
	delW := httptest.NewRecorder()
	DeleteAPIToken(delW, delReq)

	if delW.Code != http.StatusNoContent {
		t.Errorf("delete status = %d, want 204", delW.Code)
	}

	// Verify Alice's token is still there
	listReq := httptest.NewRequest("GET", "/api/v1/auth/tokens", nil)
	listReq = listReq.WithContext(middleware.WithUser(listReq.Context(), alice))
	listW := httptest.NewRecorder()
	ListAPITokens(listW, listReq)
	var items []interface{}
	json.Unmarshal(listW.Body.Bytes(), &items)
	if len(items) != 1 {
		t.Errorf("alice's token list len = %d, want 1 after bob's cross-user delete attempt", len(items))
	}
}

func TestDeleteAPIToken_InvalidID(t *testing.T) {
	setupTokensTestDB(t)
	user := createUserWithPassword(t, "alice", "p", "admin")

	req := httptest.NewRequest("DELETE", "/api/v1/auth/tokens/notanumber", nil)
	req = withChiAndUser(req, user, map[string]string{"id": "notanumber"})
	w := httptest.NewRecorder()
	DeleteAPIToken(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}
