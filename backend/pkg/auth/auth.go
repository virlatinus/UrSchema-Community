// Package auth implements UrSchema's optional single-password login.
//
// When URSCHEMA_PASSWORD is set, every /api/ request needs a session cookie
// obtained from POST /api/auth/login. Sessions live in memory, so restarting
// the server logs everyone out.
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"
)

const (
	cookieName = "urschema_session"
	sessionTTL = 12 * time.Hour
	failDelay  = time.Second
)

// Guard checks the configured password and tracks login sessions.
// A nil *Guard means authentication is disabled.
type Guard struct {
	passwordHash [32]byte

	mu       sync.Mutex
	sessions map[string]time.Time // token -> expiry

	// loginMu serializes login attempts; failed attempts hold it for
	// failDelay, capping brute force at one guess per second.
	loginMu sync.Mutex
}

// New returns a Guard for password, or nil when password is empty.
func New(password string) *Guard {
	if password == "" {
		return nil
	}
	return &Guard{
		passwordHash: sha256.Sum256([]byte(password)),
		sessions:     make(map[string]time.Time),
	}
}

// Enabled reports whether a password is required.
func (g *Guard) Enabled() bool { return g != nil }

// Authenticated reports whether r carries a valid session cookie.
// Always true when auth is disabled.
func (g *Guard) Authenticated(r *http.Request) bool {
	if g == nil {
		return true
	}
	c, err := r.Cookie(cookieName)
	if err != nil {
		return false
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	exp, ok := g.sessions[c.Value]
	if !ok {
		return false
	}
	if time.Now().After(exp) {
		delete(g.sessions, c.Value)
		return false
	}
	return true
}

// Require wraps next so it only runs for authenticated requests.
func (g *Guard) Require(next http.Handler) http.Handler {
	if g == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !g.Authenticated(r) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]any{"code": http.StatusUnauthorized, "msg": "authentication required"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

// HandleLogin handles POST /api/auth/login with body {"password": "..."}.
func (g *Guard) HandleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if g == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	var body struct {
		Password string `json:"password"`
	}
	_ = json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&body)

	g.loginMu.Lock()
	given := sha256.Sum256([]byte(body.Password))
	if subtle.ConstantTimeCompare(given[:], g.passwordHash[:]) != 1 {
		log.Printf("[auth] failed login from %s", clientAddr(r))
		time.Sleep(failDelay)
		g.loginMu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]any{"code": http.StatusUnauthorized, "msg": "invalid password"})
		return
	}
	g.loginMu.Unlock()

	token, err := newToken()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	now := time.Now()
	g.mu.Lock()
	for t, exp := range g.sessions {
		if now.After(exp) {
			delete(g.sessions, t)
		}
	}
	g.sessions[token] = now.Add(sessionTTL)
	g.mu.Unlock()
	log.Printf("[auth] login from %s", clientAddr(r))

	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   int(sessionTTL.Seconds()),
		HttpOnly: true,
		Secure:   isHTTPS(r),
		SameSite: http.SameSiteStrictMode,
	})
	w.WriteHeader(http.StatusNoContent)
}

// HandleLogout handles POST /api/auth/logout.
func (g *Guard) HandleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if g != nil {
		if c, err := r.Cookie(cookieName); err == nil {
			g.mu.Lock()
			delete(g.sessions, c.Value)
			g.mu.Unlock()
			log.Printf("[auth] logout from %s", clientAddr(r))
		}
	}
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   isHTTPS(r),
		SameSite: http.SameSiteStrictMode,
	})
	w.WriteHeader(http.StatusNoContent)
}

func newToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// clientAddr describes where a request came from for the auth log. The
// X-Forwarded-For header is included when present but marked as such: it's
// client-supplied unless a trusted proxy sets it.
func clientAddr(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		return r.RemoteAddr + " (X-Forwarded-For: " + fwd + ")"
	}
	return r.RemoteAddr
}

func isHTTPS(r *http.Request) bool {
	return r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
}
