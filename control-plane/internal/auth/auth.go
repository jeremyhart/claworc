package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const (
	SessionDuration = 3 * time.Hour
	SessionCookie   = "claworc_session"
	BcryptCost      = 12
)

func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), BcryptCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func CheckPassword(password, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

type sessionEntry struct {
	UserID    uint
	ExpiresAt time.Time
}

type SessionStore struct {
	mu       sync.RWMutex
	sessions map[string]sessionEntry
}

func NewSessionStore() *SessionStore {
	return &SessionStore{
		sessions: make(map[string]sessionEntry),
	}
}

func (s *SessionStore) Create(userID uint) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	id := hex.EncodeToString(b)
	s.mu.Lock()
	s.sessions[id] = sessionEntry{
		UserID:    userID,
		ExpiresAt: time.Now().Add(SessionDuration),
	}
	s.mu.Unlock()
	return id, nil
}

func (s *SessionStore) Get(sessionID string) (uint, bool) {
	s.mu.RLock()
	entry, ok := s.sessions[sessionID]
	s.mu.RUnlock()
	if !ok || time.Now().After(entry.ExpiresAt) {
		return 0, false
	}
	return entry.UserID, true
}

func (s *SessionStore) Delete(sessionID string) {
	s.mu.Lock()
	delete(s.sessions, sessionID)
	s.mu.Unlock()
}

func (s *SessionStore) DeleteByUserID(userID uint) {
	s.mu.Lock()
	for id, entry := range s.sessions {
		if entry.UserID == userID {
			delete(s.sessions, id)
		}
	}
	s.mu.Unlock()
}

func (s *SessionStore) DeleteByUserIDExcept(userID uint, exceptSessionID string) {
	s.mu.Lock()
	for id, entry := range s.sessions {
		if entry.UserID == userID && id != exceptSessionID {
			delete(s.sessions, id)
		}
	}
	s.mu.Unlock()
}

// GenerateAPIToken creates a new plaintext API token together with its
// SHA-256 hash (for storage) and a short display prefix.
//
//   - plain:  "claworc_pat_" + 64 hex chars (32 random bytes)
//   - hash:   hex(sha256(plain))   — the only value persisted in the DB
//   - prefix: "claworc_pat_" + first 6 chars of the hex random portion
func GenerateAPIToken() (plain, hash, prefix string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", "", "", fmt.Errorf("generate api token: %w", err)
	}
	hexRandom := hex.EncodeToString(b)
	plain = "claworc_pat_" + hexRandom
	sum := sha256.Sum256([]byte(plain))
	hash = hex.EncodeToString(sum[:])
	prefix = "claworc_pat_" + hexRandom[:6]
	return plain, hash, prefix, nil
}

func (s *SessionStore) Cleanup() {
	now := time.Now()
	s.mu.Lock()
	for id, entry := range s.sessions {
		if now.After(entry.ExpiresAt) {
			delete(s.sessions, id)
		}
	}
	s.mu.Unlock()
}
