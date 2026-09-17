package oauth

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
)

// CallbackResult is what the loopback server delivers when the OAuth provider
// redirects back. Code is the authorization code to exchange; State is the
// CSRF state the provider echoed back (must match what we sent).
type CallbackResult struct {
	Code  string
	State string
	Err   error // non-nil if the provider sent an error or state mismatched
}

// CallbackServer is a transient HTTP server that listens on localhost for the
// OAuth redirect. It exists for the duration of one authorization-code flow:
// Start it, open the authorize URL, wait on Result(), then it shuts down.
type CallbackServer struct {
	server   *http.Server
	port     int
	path     string
	host     string
	expected string
	resultCh chan CallbackResult
}

// NewCallbackServer creates a server bound to localhost:port. expectedState is
// the CSRF state we sent in the authorize request; the callback is rejected
// if the provider's echo does not match. path is the URL path to register
// (e.g. "/callback", "/auth/callback"). host is the hostname used in the
// redirect URI ("localhost" or "127.0.0.1").
func NewCallbackServer(port int, path, host, expectedState string) *CallbackServer {
	if path == "" {
		path = "/callback"
	}
	if host == "" {
		host = "localhost"
	}
	cs := &CallbackServer{
		port:     port,
		path:     path,
		host:     host,
		expected: expectedState,
		resultCh: make(chan CallbackResult, 1),
	}
	mux := http.NewServeMux()
	mux.HandleFunc(path, cs.handle)
	cs.server = &http.Server{
		Handler:  mux,
		ErrorLog: nil,
	}
	return cs
}

// Start binds the listener and begins serving. It returns the full redirect
// URI the provider should use (http://localhost:{port}/callback).
func (cs *CallbackServer) Start() (string, error) {
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", cs.port))
	if err != nil {
		return "", fmt.Errorf("oauth: bind localhost:%d: %w", cs.port, err)
	}
	cs.port = ln.Addr().(*net.TCPAddr).Port
	go func() {
		_ = cs.server.Serve(ln)
	}()
	return fmt.Sprintf("http://%s:%d%s", cs.host, cs.port, cs.path), nil
}

// Port returns the actual bound port (useful if Start picked a random one,
// though Anthropic does not allow port fallback).
func (cs *CallbackServer) Port() int { return cs.port }

func (cs *CallbackServer) handle(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	// OAuth error from the provider (user denied, server error, etc.)
	if errVal := q.Get("error"); errVal != "" {
		desc := q.Get("error_description")
		cs.deliver(w, CallbackResult{Err: fmt.Errorf("oauth: provider error: %s: %s", errVal, desc)})
		return
	}
	code := q.Get("code")
	state := q.Get("state")
	if code == "" {
		cs.deliver(w, CallbackResult{Err: errors.New("oauth: callback missing code parameter")})
		return
	}
	if state != cs.expected {
		cs.deliver(w, CallbackResult{Err: fmt.Errorf("oauth: state mismatch (expected %q, got %q)", cs.expected, state)})
		return
	}
	cs.deliver(w, CallbackResult{Code: code, State: state})
}

func (cs *CallbackServer) deliver(w http.ResponseWriter, res CallbackResult) {
	select {
	case cs.resultCh <- res:
	default:
	}
	// Show a result page in the browser. The user sees this after the
	// provider redirects; the app has already captured the code.
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if res.Err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, `<html><body><h2>Login failed</h2><p>%s</p><p>You can close this tab.</p></body></html>`, urlEscape(res.Err.Error()))
	} else {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `<html><body><h2>Login successful</h2><p>You can close this tab and return to nib.</p></body></html>`)
	}
}

// Wait blocks until the callback is received or ctx is cancelled. The server
// is shut down before returning.
func (cs *CallbackServer) Wait(ctx context.Context) CallbackResult {
	select {
	case res := <-cs.resultCh:
		cs.shutdown()
		return res
	case <-ctx.Done():
		cs.shutdown()
		return CallbackResult{Err: ctx.Err()}
	}
}

func (cs *CallbackServer) shutdown() {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_ = cs.server.Shutdown(ctx)
}

func urlEscape(s string) string { return url.QueryEscape(s) }
