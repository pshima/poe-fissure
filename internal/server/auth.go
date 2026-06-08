package server

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// Auth gates the API behind a single shared password. On success it issues a
// stateless, HMAC-signed session cookie; there is no server-side session store.
type Auth struct {
	// PasswordHash is a bcrypt hash of the site password (APP_PASSWORD_HASH).
	PasswordHash []byte
	// Secret signs session cookies (SESSION_SECRET). Keep it private.
	Secret []byte
	// TTL is how long a login lasts before re-auth is required.
	TTL time.Duration
	// Secure marks the cookie Secure (set false only for local http testing).
	Secure bool
}

const cookieName = "poefissure_session"

// errUnauthorized is returned when a request lacks a valid session.
var errUnauthorized = errors.New("unauthorized")

// sign returns "<expUnix>.<hex hmac>" for the given expiry.
func (a *Auth) sign(exp int64) string {
	msg := strconv.FormatInt(exp, 10)
	mac := hmac.New(sha256.New, a.Secret)
	mac.Write([]byte(msg))
	return msg + "." + hex.EncodeToString(mac.Sum(nil))
}

// verify checks a cookie value's signature and expiry.
func (a *Auth) verify(value string) bool {
	msg, sig, ok := strings.Cut(value, ".")
	if !ok {
		return false
	}
	exp, err := strconv.ParseInt(msg, 10, 64)
	if err != nil || time.Now().Unix() > exp {
		return false
	}
	mac := hmac.New(sha256.New, a.Secret)
	mac.Write([]byte(msg))
	want := mac.Sum(nil)
	got, err := hex.DecodeString(sig)
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare(want, got) == 1
}

// CheckPassword reports whether password matches the configured hash.
func (a *Auth) CheckPassword(password string) bool {
	return bcrypt.CompareHashAndPassword(a.PasswordHash, []byte(password)) == nil
}

// issue writes a fresh session cookie.
func (a *Auth) issue(w http.ResponseWriter) {
	exp := time.Now().Add(a.TTL)
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    a.sign(exp.Unix()),
		Path:     "/",
		Expires:  exp,
		HttpOnly: true,
		Secure:   a.Secure,
		SameSite: http.SameSiteLaxMode,
	})
}

// clear expires the session cookie.
func (a *Auth) clear(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   a.Secure,
		SameSite: http.SameSiteLaxMode,
	})
}

// authed reports whether a request carries a valid session cookie.
func (a *Auth) authed(r *http.Request) bool {
	c, err := r.Cookie(cookieName)
	if err != nil {
		return false
	}
	return a.verify(c.Value)
}

// handleLogin validates the posted password and sets a session cookie.
func (a *Auth) handleLogin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if !a.CheckPassword(body.Password) {
		writeError(w, http.StatusUnauthorized, "incorrect password")
		return
	}
	a.issue(w)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// handleLogout clears the session.
func (a *Auth) handleLogout(w http.ResponseWriter, r *http.Request) {
	a.clear(w)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// require wraps a handler so it only runs for authenticated requests.
func (a *Auth) require(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !a.authed(r) {
			writeError(w, http.StatusUnauthorized, "login required")
			return
		}
		next(w, r)
	}
}

// HashPassword returns a bcrypt hash for password (used by the `hash` subcommand).
func HashPassword(password string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(h), err
}
