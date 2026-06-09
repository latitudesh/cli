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

	session, err := client.CreateSession(ctx, authclient.CreateSessionRequest{
		ClientName:    "lsh",
		ClientVersion: version.Version,
	})
	if err != nil {
		return fmt.Errorf("could not start login session: %w", err)
	}

	headless := browser.LooksHeadless()
	printAuthorizePrompt(session, headless)

	if !headless {
		if err := browser.Open(session.AuthorizeURL); err != nil {
			fmt.Fprintln(os.Stderr, "Could not open browser automatically; please open the URL above manually.")
		}
	}

	approved, err := pollUntilApproved(ctx, client, session.ID, session.Secret)
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
func pollUntilApproved(ctx context.Context, client *authclient.Client, id, secret string) (*authclient.Session, error) {
	start := time.Now()
	deadline := start.Add(loginTimeout)
	interval := pollInterval

	for {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		session, err := client.PollSession(ctx, id, secret)
		if err == nil {
			// A successful response means the server is healthy; reset the
			// interval so a previous transient error doesn't keep us polling
			// at the backed-off rate for the rest of the session.
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
					return nil, errors.New("login session secret rejected (this should not happen)")
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

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(interval):
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
