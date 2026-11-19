package cli

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"
)

func TestLoopbackReceiverDeliversCode(t *testing.T) {
	receiver, err := startLoopbackReceiver()
	if err != nil {
		t.Fatalf("startLoopbackReceiver: %v", err)
	}
	defer receiver.close()

	resp, err := http.Get(receiver.callbackURL() + "&approval_code=the-code")
	if err != nil {
		t.Fatalf("GET callback: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	code, err := receiver.waitForCode(ctx)
	if err != nil {
		t.Fatalf("waitForCode: %v", err)
	}
	if code != "the-code" {
		t.Fatalf("expected code %q, got %q", "the-code", code)
	}
}

func TestLoopbackReceiverRejectsWrongState(t *testing.T) {
	receiver, err := startLoopbackReceiver()
	if err != nil {
		t.Fatalf("startLoopbackReceiver: %v", err)
	}
	defer receiver.close()

	url := fmt.Sprintf("http://%s/callback?state=forged&approval_code=the-code", receiver.listener.Addr())
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET callback: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for a forged state, got %d", resp.StatusCode)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	if _, err := receiver.waitForCode(ctx); err == nil {
		t.Fatal("waitForCode returned a code for a forged state")
	}
}

func TestLoopbackReceiverRejectsMissingCode(t *testing.T) {
	receiver, err := startLoopbackReceiver()
	if err != nil {
		t.Fatalf("startLoopbackReceiver: %v", err)
	}
	defer receiver.close()

	resp, err := http.Get(receiver.callbackURL())
	if err != nil {
		t.Fatalf("GET callback: %v", err)
	}
	defer io.Copy(io.Discard, resp.Body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for a missing code, got %d", resp.StatusCode)
	}
}
