package main

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"qoder2api/account"
	"qoder2api/logger"
)

const (
	sessionCookieName = "qoder2api_session"
	sessionTTL        = 24 * time.Hour
)

type sessionStore struct {
	mu   sync.Mutex
	data map[string]time.Time // token -> expire
}

var sessions = &sessionStore{data: map[string]time.Time{}}

func (s *sessionStore) create() (string, time.Time) {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	tok := hex.EncodeToString(b)
	exp := time.Now().Add(sessionTTL)
	s.mu.Lock()
	s.data[tok] = exp
	s.mu.Unlock()
	return tok, exp
}

func (s *sessionStore) valid(tok string) bool {
	if tok == "" {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	exp, ok := s.data[tok]
	if !ok {
		return false
	}
	if time.Now().After(exp) {
		delete(s.data, tok)
		return false
	}
	// sliding expiration
	s.data[tok] = time.Now().Add(sessionTTL)
	return true
}

func (s *sessionStore) revoke(tok string) {
	s.mu.Lock()
	delete(s.data, tok)
	s.mu.Unlock()
}

func effectiveConsolePassword() string {
	if v := strings.TrimSpace(os.Getenv("QODER2API_CONSOLE_PASSWORD")); v != "" {
		return v
	}
	if svc != nil {
		if st, err := svc.GetSettings(); err == nil && st != nil {
			if p := strings.TrimSpace(st.ConsolePassword); p != "" {
				return p
			}
		}
	}
	return ""
}

// ensureConsolePassword 若未配置密码则生成并写入 settings，启动时打印一次。
func ensureConsolePassword() string {
	if p := effectiveConsolePassword(); p != "" {
		return p
	}
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	p := hex.EncodeToString(b)
	cur, err := account.LoadSettings()
	if err != nil || cur == nil {
		cur = &account.Settings{Port: 8963, LogLevel: "info"}
	}
	cur.ConsolePassword = p
	_ = account.SaveSettings(cur)
	if svc != nil {
		svc.bridgeToken = cur.BridgeToken
		if cur.Port > 0 {
			svc.bridgePort = cur.Port
		}
	}
	logger.Info("console password auto-generated (saved to settings): %s", p)
	logger.Info("override with env QODER2API_CONSOLE_PASSWORD or settings.console_password")
	return p
}

func passwordOK(input string) bool {
	want := effectiveConsolePassword()
	if want == "" {
		return false
	}
	// constant-time compare
	a := sha256.Sum256([]byte(input))
	b := sha256.Sum256([]byte(want))
	return subtle.ConstantTimeCompare(a[:], b[:]) == 1
}

func sessionTokenFromRequest(r *http.Request) string {
	if c, err := r.Cookie(sessionCookieName); err == nil && c != nil {
		return c.Value
	}
	// also allow Authorization: Bearer <session> for API tools
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		return strings.TrimSpace(auth[7:])
	}
	return ""
}

func setSessionCookie(w http.ResponseWriter, tok string, exp time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    tok,
		Path:     "/",
		Expires:  exp,
		MaxAge:   int(sessionTTL.Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		// Secure: only enable behind HTTPS; leave false for local/http deploy
	})
}

func clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func isPublicPath(path string) bool {
	if path == "/login.html" || path == "/favicon.ico" {
		return true
	}
	if strings.HasPrefix(path, "/api/auth/") {
		return true
	}
	// static assets for login page
	if strings.HasPrefix(path, "/assets/") {
		return true
	}
	return false
}

func requireConsoleAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}
		if isPublicPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		tok := sessionTokenFromRequest(r)
		if sessions.valid(tok) {
			next.ServeHTTP(w, r)
			return
		}
		// HTML pages -> redirect login; API -> 401 JSON
		if strings.HasPrefix(r.URL.Path, "/api/") {
			writeError(w, http.StatusUnauthorized, fmt.Errorf("unauthorized: please login"))
			return
		}
		http.Redirect(w, r, "/login.html", http.StatusFound)
	})
}

func handleAuthLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("POST required"))
		return
	}
	var req struct {
		Password string `json:"password"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if !passwordOK(strings.TrimSpace(req.Password)) {
		// small delay against brute force
		time.Sleep(300 * time.Millisecond)
		writeError(w, http.StatusUnauthorized, fmt.Errorf("invalid password"))
		return
	}
	tok, exp := sessions.create()
	setSessionCookie(w, tok, exp)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":         true,
		"expires_at": exp.Unix(),
	})
}

func handleAuthLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("POST required"))
		return
	}
	tok := sessionTokenFromRequest(r)
	sessions.revoke(tok)
	clearSessionCookie(w)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func handleAuthMe(w http.ResponseWriter, r *http.Request) {
	tok := sessionTokenFromRequest(r)
	ok := sessions.valid(tok)
	if !ok {
		writeError(w, http.StatusUnauthorized, fmt.Errorf("unauthorized"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":      true,
		"authed":  true,
		"message": "console session active",
	})
}
