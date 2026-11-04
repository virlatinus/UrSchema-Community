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
	"math"
	"net"
	"net/http"
	"sqlmanager/pkg/proxy"
	"strconv"
	"sync"
	"time"
)

const (
	cookieName = "urschema_session"
	sessionTTL = 12 * time.Hour

	// After a failed login, the same client address must wait
	// backoffBase, doubling with each further failure up to backoffMax.
	// A client's failures are forgotten forgetAfter its back-off ends.
	backoffBase = time.Second
	backoffMax  = 5 * time.Minute
	forgetAfter = 15 * time.Minute
)

// Guard checks the configured password and tracks login sessions.
// A nil *Guard means authentication is disabled.
type Guard struct {
	passwordHash [32]byte
	cookiePath   string

	mu       sync.Mutex
	sessions map[string]time.Time // token -> expiry

	// failMu guards failures and is held for each whole login decision
	// (a cheap hash compare), so parallel requests can't skip a back-off.
	failMu   sync.Mutex
	failures map[string]*failState // client IP -> recent failed logins
}

type failState struct {
	count int
	until time.Time // no attempts from this client before this time
}

// New returns a Guard for password, or nil when password is empty.
// cookiePath limits the session cookie to UrSchema's own pages, e.g.
// "/db-admin" when it is served under that prefix.
func New(password, cookiePath string) *Guard {
	if password == "" {
		return nil
	}
	return &Guard{
		passwordHash: sha256.Sum256([]byte(password)),
		cookiePath:   cookiePath,
		sessions:     make(map[string]time.Time),
		failures:     make(map[string]*failState),
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

	ok, retryAfter := g.checkPassword(clientIP(r), body.Password)
	if retryAfter > 0 {
		w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(retryAfter.Seconds()))))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		_ = json.NewEncoder(w).Encode(map[string]any{"code": http.StatusTooManyRequests, "msg": "too many failed logins, try again later"})
		return
	}
	if !ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]any{"code": http.StatusUnauthorized, "msg": "invalid password"})
		return
	}

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
		Path:     g.cookiePath,
		MaxAge:   int(sessionTTL.Seconds()),
		HttpOnly: true,
		Secure:   proxy.HTTPS(r),
		SameSite: http.SameSiteStrictMode,
	})
	w.WriteHeader(http.StatusNoContent)
}

// checkPassword verifies password for a client. A client still backing off
// from earlier failures gets retryAfter > 0 and no check at all. Failures
// only slow down the address they come from; a correct password from any
// other address is accepted immediately.
func (g *Guard) checkPassword(ip, password string) (ok bool, retryAfter time.Duration) {
	g.failMu.Lock()
	defer g.failMu.Unlock()

	now := time.Now()
	for addr, f := range g.failures {
		if now.After(f.until.Add(forgetAfter)) {
			delete(g.failures, addr)
		}
	}

	f := g.failures[ip]
	if f != nil && now.Before(f.until) {
		return false, f.until.Sub(now)
	}

	given := sha256.Sum256([]byte(password))
	if subtle.ConstantTimeCompare(given[:], g.passwordHash[:]) == 1 {
		delete(g.failures, ip)
		return true, 0
	}

	if f == nil {
		f = &failState{}
		g.failures[ip] = f
	}
	f.count++
	backoff := backoffMax
	if f.count <= 20 { // beyond that the shift would overflow; already capped
		backoff = min(backoffBase<<(f.count-1), backoffMax)
	}
	f.until = now.Add(backoff)
	log.Printf("[auth] failed login from %s (%d in a row; next attempt allowed in %s)", ip, f.count, backoff)
	return false, 0
}

// clientIP is the address failed logins are counted against, without the
// port: the direct peer, or the client behind a trusted proxy (see
// proxy.Trusted.Wrap). Behind an untrusted proxy every client shares the
// proxy's address.
func clientIP(r *http.Request) string {
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}

// HandleLogout handles POST /api/auth/logout.
func (g *Guard) HandleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if g == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if c, err := r.Cookie(cookieName); err == nil {
		g.mu.Lock()
		delete(g.sessions, c.Value)
		g.mu.Unlock()
		log.Printf("[auth] logout from %s", clientAddr(r))
	}
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     g.cookiePath,
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   proxy.HTTPS(r),
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
