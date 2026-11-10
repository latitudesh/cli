package cli

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"
)

// loopbackReceiver is a one-shot local HTTP server that receives the
// approval code the dashboard redirects to after a browser login is
// approved. Binding to 127.0.0.1 means the code reaches this machine
// only, so a forwarded approval link cannot deliver it to the session's
// creator (RFC 8252 loopback redirect).
type loopbackReceiver struct {
	listener net.Listener
	server   *http.Server
	state    string
	resultCh chan string
	errCh    chan error
}

func startLoopbackReceiver() (*loopbackReceiver, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("could not open a local callback port: %w", err)
	}

	stateBytes := make([]byte, 16)
	if _, err := rand.Read(stateBytes); err != nil {
		listener.Close()
		return nil, fmt.Errorf("could not generate callback state: %w", err)
	}

	receiver := &loopbackReceiver{
		listener: listener,
		state:    hex.EncodeToString(stateBytes),
		resultCh: make(chan string, 1),
		errCh:    make(chan error, 1),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", receiver.handleCallback)
	receiver.server = &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}

	go func() {
		if err := receiver.server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			receiver.errCh <- err
		}
	}()

	return receiver, nil
}

func (r *loopbackReceiver) callbackURL() string {
	callback := url.URL{
		Scheme:   "http",
		Host:     r.listener.Addr().String(),
		Path:     "/callback",
		RawQuery: url.Values{"state": {r.state}}.Encode(),
	}
	return callback.String()
}

func (r *loopbackReceiver) handleCallback(w http.ResponseWriter, req *http.Request) {
	query := req.URL.Query()
	if subtle.ConstantTimeCompare([]byte(query.Get("state")), []byte(r.state)) != 1 {
		http.Error(w, "unexpected login callback", http.StatusBadRequest)
		return
	}

	code := query.Get("approval_code")
	if code == "" {
		http.Error(w, "login callback is missing its approval code", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, loopbackSuccessHTML)

	select {
	case r.resultCh <- code:
	default:
	}
}

func (r *loopbackReceiver) waitForCode(ctx context.Context) (string, error) {
	select {
	case code := <-r.resultCh:
		return code, nil
	case err := <-r.errCh:
		return "", err
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

func (r *loopbackReceiver) close() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_ = r.server.Shutdown(ctx)
}

const loopbackSuccessHTML = `<!doctype html>
<html lang="en">
<head><meta charset="utf-8"><title>Login complete</title></head>
<body style="font-family:system-ui,sans-serif;text-align:center;padding-top:4rem">
<h2>You're all set</h2>
<p>Your CLI is now authorized. You can close this tab and return to your terminal.</p>
</body>
</html>`
