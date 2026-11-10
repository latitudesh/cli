package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/latitudesh/lsh/internal/authclient"
	"github.com/latitudesh/lsh/internal/browser"
	"github.com/latitudesh/lsh/internal/config"
	"github.com/latitudesh/lsh/internal/version"
)

// looksHeadless is a seam so tests can exercise both the loopback and the
// secret-only login paths regardless of the host environment.
var looksHeadless = browser.LooksHeadless

const (
	pollInterval   = 2 * time.Second
	pollMaxBackoff = 5 * time.Second
	loginTimeout   = 5*time.Minute + 30*time.Second // a touch over the API TTL

	// Window after CreateSession during which a 404 from the poll
	// endpoint is treated as "not yet visible" instead of "expired".
	// Covers initial Redis propagation and the gap between the create
	// returning and the dashboard/CLI being able to read the session.
	earlyNotFoundGrace = 15 * time.Second
)

func loginViaBrowser(ctx context.Context, client *authclient.Client, profileOverride string) error {
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt)
	defer cancel()

	headless := looksHeadless()

	req := authclient.CreateSessionRequest{
		ClientName:    "lsh",
		ClientVersion: version.Version,
	}

	// A browser login binds the session to this machine with a loopback
	// callback, so the approval code is delivered here and a forwarded
	// approval link cannot hand the credential to someone else. A headless
	// environment has no reachable loopback for the approver's browser, so
	// it falls back to the secret-only poll.
	var (
		receiver *loopbackReceiver
		err      error
	)
	if !headless {
		receiver, err = startLoopbackReceiver()
		if err != nil {
			return fmt.Errorf("could not start the local login listener: %w", err)
		}
		defer receiver.close()
		req.CallbackURL = receiver.callbackURL()
	}

	session, err := client.CreateSession(ctx, req)
	if err != nil {
		return fmt.Errorf("could not start login session: %w", err)
	}

	printAuthorizePrompt(session, headless)

	if !headless {
		if err := browser.Open(session.AuthorizeURL); err != nil {
			fmt.Fprintln(os.Stderr, "Could not open browser automatically; please open the URL above manually.")
		}
	}

	var (
		codeCh <-chan string
		errCh  <-chan error
	)
	if receiver != nil {
		codeCh = receiver.resultCh
		errCh = receiver.errCh
	}

	approved, err := pollUntilApproved(ctx, client, session.ID, session.Secret, codeCh, errCh)
	if err != nil {
		return err
	}

	if approved.APIKey == nil || approved.Team == nil || approved.User == nil {
		return errors.New("login session approved but the credential payload was incomplete")
	}

	profile := config.Profile{
		Authorization: approved.APIKey.Token,
		KeyID:         approved.APIKey.ID,
		KeyName:       approved.APIKey.Name,
		TeamID:        approved.Team.ID,
		TeamName:      approved.Team.Name,
		TeamSlug:      approved.Team.Slug,
		Email:         approved.User.Email,
		Source:        config.SourceBrowser,
		APIVersion:    defaultAPIVersion(),
	}

	name, err := saveProfile(profileOverride, profile)
	if err != nil {
		return fmt.Errorf("could not save profile: %w", err)
	}

	fmt.Printf("\n✅ Logged in as %s on team %s (profile: %s)\n", profile.Email, profile.TeamName, name)
	return nil
}

func printAuthorizePrompt(session *authclient.Session, headless bool) {
	fmt.Println("Opening your browser to authorize this CLI...")
	fmt.Println()
	fmt.Println("  URL:")
	fmt.Printf("    %s\n", session.AuthorizeURL)
	fmt.Println()
	fmt.Println("  Confirm this code matches what your browser shows:")
	fmt.Printf("    %s\n", session.UserCode)
	if headless {
		fmt.Println()
		fmt.Println("  (detected headless environment — open the URL above on a machine with a browser)")
	}
	fmt.Println()
	fmt.Println("Waiting for approval... press Ctrl+C to cancel.")
}

// pollUntilApproved retries GET /auth/cli_sessions/<id> until the
// session is approved, terminally errored, or the deadline expires.
//
// For a callback-bound login, codeCh delivers the approval code once the
// loopback callback fires. Until then the loop keeps polling (with no
// code) so a cancelled or expired session is seen promptly instead of
// blocking on the callback for the whole timeout; the API returns 401
// only once the session is approved and still awaiting its code. A nil
// codeCh is the secret-only (headless) flow. errCh reports a failure of the
// local callback listener so login fails promptly instead of waiting out the
// timeout for a callback that can no longer arrive.
func pollUntilApproved(ctx context.Context, client *authclient.Client, id, secret string, codeCh <-chan string, errCh <-chan error) (*authclient.Session, error) {
	start := time.Now()
	deadline := start.Add(loginTimeout)
	interval := pollInterval

	approvalCode := ""
	awaitingCode := codeCh != nil

	for {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		session, err := client.PollSessionWithCode(ctx, id, secret, approvalCode)
		if err == nil {
			interval = pollInterval
			if session.Status == "approved" {
				if session.APIKey != nil {
					return session, nil
				}
				// Approval and key are written together server-side, so an
				// approved session with no key is anomalous — fail fast
				// instead of polling silently until the deadline.
				return nil, errors.New("login was approved but no API key was returned; please run `lsh login` again")
			}
			// status=pending → keep polling
		} else {
			var httpErr *authclient.HTTPError
			if errors.As(err, &httpErr) {
				switch httpErr.StatusCode {
				case 404:
					// 404 right after CreateSession just means the session
					// has not propagated yet — retry within the grace
					// window before treating it as terminal.
					if time.Since(start) >= earlyNotFoundGrace {
						return nil, errors.New("login session expired or was cancelled")
					}
				case 410:
					return nil, errors.New("login session has already been used; please run `lsh login` again")
				case 401:
					// Once approved, a callback session returns 401 until the
					// approval code arrives — keep waiting for it. Without a
					// pending code the secret itself was rejected.
					if !awaitingCode {
						return nil, errors.New("login session secret rejected (this should not happen)")
					}
				default:
					// transient — back off and retry until deadline
				}
			} else {
				// network error — back off and retry
			}
			interval = nextBackoff(interval)
		}

		if time.Now().After(deadline) {
			return nil, errors.New("login session expired before approval")
		}

		if awaitingCode {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case approvalCode = <-codeCh:
				awaitingCode = false
				interval = pollInterval
			case err := <-errCh:
				return nil, fmt.Errorf("the local login listener failed: %w", err)
			case <-time.After(interval):
			}
		} else {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(interval):
			}
		}
	}
}

func nextBackoff(current time.Duration) time.Duration {
	doubled := current * 2
	if doubled > pollMaxBackoff {
		return pollMaxBackoff
	}
	return doubled
}
