package objectstorage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/latitudesh/latitudesh-go-sdk/models/components"
	"github.com/latitudesh/lsh/internal/exitcode"
	"github.com/latitudesh/lsh/internal/tui"
	"github.com/minio/minio-go/v7"
)

// Humanize rewrites an S3 or API error into an actionable message with the
// right exit code. Errors that already carry an exit code pass through.
func Humanize(err error, b *Bucket, c *Credential) error {
	if err == nil {
		return nil
	}
	var already *exitcode.Error
	if errors.As(err, &already) {
		return err
	}
	if errors.Is(err, context.Canceled) {
		return exitcode.Errorf(exitcode.Interrupted, "interrupted")
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return exitcode.Errorf(exitcode.Generic, "timed out: %v", err)
	}

	bucketName := ""
	if b != nil {
		bucketName = b.Display()
	}
	cred := "the credential"
	if c != nil {
		if b != nil {
			cred = c.Describe(b.ID)
		} else {
			cred = c.Source
		}
	}

	if resp := minio.ToErrorResponse(err); resp.Code != "" || resp.StatusCode != 0 {
		switch resp.Code {
		case "NoSuchBucket":
			endpoint := ""
			backend := resp.BucketName
			if b != nil {
				endpoint = b.Endpoint
				backend = b.BucketName
			}
			return exitcode.Errorf(exitcode.NotFound, "bucket %s (backend name %q) was not found at %s; it may still be provisioning — check 'lsh s3 stat %s'", bucketName, backend, endpoint, displayOrBackend(b))
		case "NoSuchKey", "NotFound":
			key := resp.Key
			if key == "" {
				return exitcode.Errorf(exitcode.NotFound, "object not found in bucket %s", bucketName)
			}
			return exitcode.Errorf(exitcode.NotFound, "object %q not found in bucket %s", key, bucketName)
		case "AccessDenied", "AllAccessDisabled":
			hint := ""
			if b != nil {
				hint = fmt.Sprintf("; use a key with rw permission (lsh s3 access-keys list) or create one: lsh s3 access-keys create --bucket %s --save", displayOrBackend(b))
			}
			return exitcode.Errorf(exitcode.Permission, "access denied to %s using %s%s", bucketName, cred, hint)
		case "InvalidAccessKeyId", "SignatureDoesNotMatch", "InvalidToken", "ExpiredToken":
			hint := ""
			if c != nil && !c.FromEnv && c.Name != "" {
				hint = fmt.Sprintf("; if it was deleted or rotated, run 'lsh s3 access-keys forget %s' and create a new one", c.Name)
			}
			return exitcode.Errorf(exitcode.Credentials, "the backend rejected %s (%s)%s", cred, resp.Code, hint)
		case "AuthorizationHeaderMalformed":
			expected := resp.Region
			signed := ""
			if b != nil {
				signed = b.SigningRegion
			}
			if expected != "" {
				return exitcode.Errorf(exitcode.Credentials, "signing region mismatch: the backend expects %q but the request was signed for %q; retry with --signing-region %s (or %s=%s)", expected, signed, expected, EnvSigningRegion, expected)
			}
			return exitcode.Errorf(exitcode.Credentials, "signing region mismatch (signed for %q); retry with --signing-region <region>", signed)
		case "BucketNotEmpty":
			return exitcode.Errorf(exitcode.Refused, "bucket %s is not empty; re-run with --force to delete its objects first", bucketName)
		case "BucketAlreadyOwnedByYou", "BucketAlreadyExists":
			return exitcode.Errorf(exitcode.Usage, "bucket %s already exists", bucketName)
		case "EntityTooLarge":
			return exitcode.Errorf(exitcode.Generic, "object exceeds the backend size limit: %s", resp.Message)
		case "RequestTimeTooSkewed":
			return exitcode.Errorf(exitcode.Generic, "the local clock is off by more than 15 minutes; fix the system time and retry")
		case "InvalidArgument", "InvalidRequest", "XMinioInvalidObjectName", "KeyTooLongError", "InvalidBucketName":
			return exitcode.Errorf(exitcode.Usage, "%s: %s", resp.Code, resp.Message)
		case "ObjectLocked", "InvalidRetentionPeriod", "ObjectLockConfigurationNotFoundError":
			return exitcode.Errorf(exitcode.Refused, "%s: %s", resp.Code, resp.Message)
		case "PreconditionFailed":
			return exitcode.Errorf(exitcode.Generic, "precondition failed: %s", resp.Message)
		case "SlowDown", "ServiceUnavailable", "InternalError":
			return exitcode.Errorf(exitcode.Generic, "the backend is busy (%s); retry in a moment", resp.Code)
		}
		if resp.StatusCode == 403 {
			return exitcode.Errorf(exitcode.Permission, "access denied to %s using %s", bucketName, cred)
		}
		if resp.StatusCode == 404 {
			return exitcode.Errorf(exitcode.NotFound, "not found: %s", firstNonEmpty(resp.Message, resp.Code, "404"))
		}
		msg := firstNonEmpty(resp.Message, resp.Code)
		if resp.Code != "" && resp.Message != "" {
			msg = resp.Code + ": " + resp.Message
		}
		return exitcode.Errorf(exitcode.Generic, "%s", Redact(msg))
	}

	if e := humanizeLatitudeError(err, ""); e != nil {
		return e
	}

	var urlErr *url.Error
	var netErr net.Error
	var opErr *net.OpError
	if errors.As(err, &urlErr) || errors.As(err, &netErr) || errors.As(err, &opErr) {
		target := ""
		if b != nil {
			target = b.Endpoint
		}
		return exitcode.Errorf(exitcode.Generic, "could not reach %s: %v", target, Redact(err.Error()))
	}
	return exitcode.New(exitcode.Generic, errors.New(Redact(err.Error())))
}

func displayOrBackend(b *Bucket) string {
	if b == nil {
		return ""
	}
	if b.Name != "" {
		return b.Name
	}
	return b.BucketName
}

// humanizeAPIError maps Latitude API errors for control-plane calls.
func humanizeAPIError(err error, what string) error {
	if e := humanizeLatitudeError(err, what); e != nil {
		return e
	}
	// Same interruption contract as Humanize: the exit code must not depend on
	// whether the signal landed during an API call or an S3 call.
	if errors.Is(err, context.Canceled) {
		return exitcode.Errorf(exitcode.Interrupted, "interrupted")
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return exitcode.Errorf(exitcode.Generic, "timed out: %v", err)
	}
	var already *exitcode.Error
	if errors.As(err, &already) {
		return err
	}
	return exitcode.New(exitcode.Generic, err)
}

// HumanizeAPI is the exported form of humanizeAPIError for commands that
// only talk to the Latitude API (access keys, lifecycle, metrics, usage).
func HumanizeAPI(err error, what string) error {
	if err == nil {
		return nil
	}
	return humanizeAPIError(err, what)
}

// humanizeLatitudeError recognizes the two error shapes the SDK produces:
// *components.APIError (unmodeled statuses; carries the HTTP status and raw
// body) and *components.ErrorObject (JSON:API errors for the statuses the
// spec models — 403/404/422/500 — where the status is a string per entry).
// It returns nil for any other error.
func humanizeLatitudeError(err error, what string) error {
	var apiErr *components.APIError
	if errors.As(err, &apiErr) {
		return humanizeAPIStatus(apiErr.StatusCode, []byte(apiErr.Body), what)
	}
	var eo *components.ErrorObject
	if errors.As(err, &eo) {
		status := 0
		var parts []string
		for _, e := range eo.Errors {
			if status == 0 && e.Status != nil {
				if n, convErr := strconv.Atoi(strings.TrimSpace(*e.Status)); convErr == nil {
					status = n
				}
			}
			msg := firstNonEmpty(str(e.Detail), str(e.Title), str(e.Code))
			if msg == "" {
				continue
			}
			if e.Source != nil && e.Source.Pointer != nil && *e.Source.Pointer != "" {
				msg = strings.TrimPrefix(*e.Source.Pointer, "/data/attributes/") + " " + msg
			}
			parts = append(parts, msg)
		}
		detail := strings.Join(parts, "; ")
		if status == 0 {
			switch {
			case strings.Contains(strings.ToLower(detail), "not found"):
				status = 404
			case strings.Contains(strings.ToLower(detail), "forbidden"), strings.Contains(strings.ToLower(detail), "permission"):
				status = 403
			default:
				status = 422
			}
		}
		body, _ := json.Marshal(map[string]interface{}{"errors": []map[string]string{{"detail": detail}}})
		return humanizeAPIStatus(status, body, what)
	}
	return nil
}

// humanizeAPIStatus turns an HTTP status + JSON:API body into an exit-coded
// error with the API's own detail message when present.
func humanizeAPIStatus(status int, body []byte, what string) error {
	detail := apiDetail(body)
	switch status {
	case 401:
		return exitcode.Errorf(exitcode.Credentials, "your API token is invalid or revoked; run 'lsh login' to sign in again")
	case 403:
		return exitcode.Errorf(exitcode.Permission, "your API token does not have permission for this action%s", suffix(detail))
	case 404:
		if what != "" {
			return exitcode.Errorf(exitcode.NotFound, "%s not found%s", what, suffix(detail))
		}
		return exitcode.Errorf(exitcode.NotFound, "not found%s", suffix(detail))
	case 409:
		if strings.Contains(strings.ToLower(detail), "not empty") {
			return exitcode.Errorf(exitcode.Refused, "%s", detail)
		}
		return exitcode.Errorf(exitcode.Usage, "conflict%s", suffix(detail))
	case 422, 400:
		return exitcode.Errorf(exitcode.Usage, "the API rejected the request%s", suffix(detail))
	case 429:
		return exitcode.Errorf(exitcode.Generic, "rate limited by the API; retry in a moment")
	}
	if status >= 500 {
		return exitcode.Errorf(exitcode.Generic, "the API returned %d%s", status, suffix(detail))
	}
	return exitcode.Errorf(exitcode.Generic, "API error %d%s", status, suffix(detail))
}

func suffix(detail string) string {
	if detail == "" {
		return ""
	}
	return ": " + detail
}

// apiDetail extracts the most useful text from a JSON:API error body.
func apiDetail(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	var env struct {
		Errors []struct {
			Title  string `json:"title"`
			Detail string `json:"detail"`
			Code   string `json:"code"`
			Source struct {
				Pointer string `json:"pointer"`
			} `json:"source"`
		} `json:"errors"`
		Error   string `json:"error"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		s := strings.TrimSpace(string(body))
		if len(s) > 200 {
			s = s[:200] + "…"
		}
		return s
	}
	var parts []string
	for _, e := range env.Errors {
		msg := firstNonEmpty(e.Detail, e.Title, e.Code)
		if msg == "" {
			continue
		}
		if e.Source.Pointer != "" {
			msg = strings.TrimPrefix(e.Source.Pointer, "/data/attributes/") + " " + msg
		}
		parts = append(parts, msg)
	}
	if len(parts) > 0 {
		return strings.Join(parts, "; ")
	}
	return firstNonEmpty(env.Message, env.Error)
}

// PrintedError marks an error that was already written to stderr by
// PrintError, so the command wrapper in cmd/s3 does not print it a second
// time. It unwraps to the original error (exit codes are preserved).
type PrintedError struct{ Err error }

func (p *PrintedError) Error() string { return p.Err.Error() }

// Unwrap exposes the wrapped error to errors.Is/As.
func (p *PrintedError) Unwrap() error { return p.Err }

// PrintError writes a humanized error to stderr in the CLI's error style and
// returns it wrapped in PrintedError (with its exit code) so RunE can
// propagate it without cobra echoing it again.
func PrintError(err error) error {
	if err == nil {
		return nil
	}
	var already *PrintedError
	if errors.As(err, &already) {
		return err
	}
	fmt.Fprintln(os.Stderr, tui.ErrorStyle.Render("✗ Error: ")+err.Error())
	return &PrintedError{Err: err}
}

// IsPrinted reports whether err was already printed by PrintError.
func IsPrinted(err error) bool {
	var p *PrintedError
	return errors.As(err, &p)
}
