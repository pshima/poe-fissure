package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

const (
	authorizeURL = "https://www.pathofexile.com/oauth/authorize"
	tokenURL     = "https://www.pathofexile.com/oauth/token"
	// Scope needed to read the account's own characters and inventories.
	scopeCharacters = "account:characters"
	// pobClientID is the shared public client Path of Building ships. It has no
	// secret (it's plaintext in PoB's source), so this tool can reuse it to reach
	// the official API without its own GGG registration.
	pobClientID = "pob"
)

// pobCallbackPorts are the exact loopback ports PoB registered for the "pob"
// client. GGG exact-matches the redirect URI (it does NOT honor RFC 8252 loopback
// port flexibility), so reusing pob means binding one of these and sending the
// matching http://localhost:<port> redirect URI.
var pobCallbackPorts = []int{49082, 49083, 49084}

// Config configures the OAuth flow for a public client.
type Config struct {
	ClientID     string
	RedirectPort int
	UserAgent    string
	// HTTPClient is optional; defaults to a client with a sane timeout.
	HTTPClient *http.Client
}

// redirectURI returns the bare loopback URI (no path) for the given port. GGG
// exact-matches this against the client's registered redirect, so the port must
// be one the client actually registered and the path must stay empty.
func redirectURI(port int) string {
	return fmt.Sprintf("http://localhost:%d", port)
}

// callbackPorts lists the loopback ports to try binding, in order. For the shared
// pob client only its registered ports work; for a custom client_id we honor the
// single configured port.
func (c Config) callbackPorts() []int {
	if c.ClientID == pobClientID {
		return pobCallbackPorts
	}
	return []int{c.RedirectPort}
}

func (c Config) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return &http.Client{Timeout: 30 * time.Second}
}

// Login runs the full Authorization Code + PKCE flow: it opens the browser to
// the consent page, captures the redirect on a one-shot localhost server,
// exchanges the code for tokens, and returns them. The caller persists them.
func (c Config) Login(ctx context.Context) (Token, error) {
	if c.ClientID == "" {
		return Token{}, errors.New("no client_id configured (register a public client with oauth@grindinggear.com)")
	}
	pkce, err := NewPKCE()
	if err != nil {
		return Token{}, err
	}
	state, err := randomState()
	if err != nil {
		return Token{}, err
	}

	ports := c.callbackPorts()
	var ln net.Listener
	var boundPort int
	for _, p := range ports {
		ln, err = net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", p))
		if err == nil {
			boundPort = p
			break
		}
	}
	if ln == nil {
		return Token{}, fmt.Errorf("could not bind any OAuth callback port %v (last error: %w)", ports, err)
	}
	defer ln.Close()
	redirect := redirectURI(boundPort)

	type result struct {
		code string
		err  error
	}
	resCh := make(chan result, 1)

	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The bare loopback redirect lands on "/" (favicon and other paths 404).
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		q := r.URL.Query()
		if e := q.Get("error"); e != "" {
			io.WriteString(w, "Authorization failed. You can close this tab.")
			resCh <- result{err: fmt.Errorf("authorization error: %s %s", e, q.Get("error_description"))}
			return
		}
		if q.Get("state") != state {
			io.WriteString(w, "State mismatch. You can close this tab.")
			resCh <- result{err: errors.New("state mismatch (possible CSRF)")}
			return
		}
		io.WriteString(w, "Authorization complete. You can close this tab and return to the terminal.")
		resCh <- result{code: q.Get("code")}
	})}
	go srv.Serve(ln)
	defer srv.Close()

	authURL := c.authorizeURL(pkce.Challenge, state, redirect)
	fmt.Println("Opening browser for Path of Exile login (Steam login works here)...")
	fmt.Println("If it does not open, visit:\n" + authURL)
	_ = openBrowser(authURL)

	select {
	case <-ctx.Done():
		return Token{}, ctx.Err()
	case res := <-resCh:
		if res.err != nil {
			return Token{}, res.err
		}
		return c.exchange(ctx, res.code, pkce.Verifier, redirect)
	case <-time.After(5 * time.Minute):
		return Token{}, errors.New("timed out waiting for authorization")
	}
}

func (c Config) authorizeURL(challenge, state, redirect string) string {
	v := url.Values{}
	v.Set("client_id", c.ClientID)
	v.Set("response_type", "code")
	v.Set("scope", scopeCharacters)
	v.Set("state", state)
	v.Set("redirect_uri", redirect)
	v.Set("code_challenge", challenge)
	v.Set("code_challenge_method", "S256")
	return authorizeURL + "?" + v.Encode()
}

func (c Config) exchange(ctx context.Context, code, verifier, redirect string) (Token, error) {
	v := url.Values{}
	v.Set("client_id", c.ClientID)
	v.Set("grant_type", "authorization_code")
	v.Set("code", code)
	v.Set("redirect_uri", redirect)
	v.Set("code_verifier", verifier)
	return c.postToken(ctx, v)
}

// Refresh exchanges a refresh token for a new access token.
func (c Config) Refresh(ctx context.Context, refreshToken string) (Token, error) {
	v := url.Values{}
	v.Set("client_id", c.ClientID)
	v.Set("grant_type", "refresh_token")
	v.Set("refresh_token", refreshToken)
	return c.postToken(ctx, v)
}

func (c Config) postToken(ctx context.Context, form url.Values) (Token, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return Token{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", c.UserAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return Token{}, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return Token{}, fmt.Errorf("token endpoint returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var tr tokenResponse
	if err := json.Unmarshal(body, &tr); err != nil {
		return Token{}, fmt.Errorf("decode token response: %w", err)
	}
	return tr.toToken(time.Now()), nil
}

func openBrowser(target string) error {
	var cmd string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		cmd = "open"
	case "windows":
		cmd, args = "rundll32", []string{"url.dll,FileProtocolHandler"}
	default:
		cmd = "xdg-open"
	}
	args = append(args, target)
	return exec.Command(cmd, args...).Start()
}
