package objectstorage

import (
	"context"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"syscall"
	"time"

	"github.com/latitudesh/lsh/internal/exitcode"
	"github.com/latitudesh/lsh/internal/version"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// Addressing styles for the escape hatch flag.
const (
	AddressingPath    = "path"
	AddressingVirtual = "virtual"
)

// ClientOptions tunes NewS3Client.
type ClientOptions struct {
	// Debug enables HTTP tracing (redacted) to Trace (stderr when nil).
	Debug bool
	Trace io.Writer
	// Addressing is path (default) or virtual.
	Addressing string
	// MaxRetries caps request retries (minio default 10 when zero).
	MaxRetries int
}

// NewS3Client builds the minio client for a bucket with the settings every
// S3-compatible backend needs: explicit endpoint and signing region (never
// discovered through GetBucketLocation), path-style addressing, no trailing
// checksums (Content-MD5 is used instead), sane timeouts without a total
// deadline so long uploads are not killed.
func NewS3Client(b *Bucket, c Credential, o ClientOptions) (*minio.Client, error) {
	if err := b.Validate(); err != nil {
		return nil, err
	}
	host, secure, err := EndpointHost(b.Endpoint)
	if err != nil {
		return nil, exitcode.New(exitcode.Usage, err)
	}
	lookup := minio.BucketLookupPath
	if o.Addressing == AddressingVirtual {
		lookup = minio.BucketLookupDNS
	}
	region := b.SigningRegion
	if region == "" {
		region = DefaultSigningRegion
	}
	client, err := minio.New(host, &minio.Options{
		Creds:           credentials.NewStaticV4(c.AccessKeyID, c.Secret(), ""),
		Secure:          secure,
		Region:          region,
		BucketLookup:    lookup,
		TrailingHeaders: false,
		Transport:       newTransport(),
		MaxRetries:      o.MaxRetries,
	})
	if err != nil {
		return nil, exitcode.Errorf(exitcode.Generic, "could not create S3 client for %s: %v", host, err)
	}
	client.SetAppInfo("lsh", version.Version)
	if o.Debug {
		w := o.Trace
		if w == nil {
			w = os.Stderr
		}
		client.TraceOn(&redactingWriter{w: w})
	}
	return client, nil
}

// newTransport mirrors the minio default transport but with explicit
// connection timeouts and no overall request deadline.
func newTransport() http.RoundTripper {
	return &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   15 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          64,
		MaxIdleConnsPerHost:   16,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   15 * time.Second,
		ResponseHeaderTimeout: 90 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ForceAttemptHTTP2:     true,
		DisableCompression:    true,
	}
}

var redactPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(Signature=)[0-9a-f]+`),
	regexp.MustCompile(`(?i)(X-Amz-Signature=)[0-9a-f]+`),
	regexp.MustCompile(`(?i)(X-Amz-Security-Token=)[^&\s]+`),
	regexp.MustCompile(`(?i)(x-amz-security-token: )\S+`),
	regexp.MustCompile(`(?i)("?secret_access_key"?\s*[:=]\s*"?)[^",\s]+`),
	regexp.MustCompile(`(?i)("?secret_key"?\s*[:=]\s*"?)[^",\s]+`),
}

// redactingWriter strips signatures and secrets from minio's HTTP trace.
type redactingWriter struct{ w io.Writer }

func (r *redactingWriter) Write(p []byte) (int, error) {
	out := p
	for _, re := range redactPatterns {
		out = re.ReplaceAll(out, []byte("${1}[redacted]"))
	}
	if _, err := r.w.Write(out); err != nil {
		return 0, err
	}
	return len(p), nil
}

// Redact applies the same redaction to an arbitrary string (error messages,
// debug lines).
func Redact(s string) string {
	for _, re := range redactPatterns {
		s = re.ReplaceAllString(s, "${1}[redacted]")
	}
	return s
}

// SignalContext returns a context cancelled on SIGINT/SIGTERM so transfers can
// abort cleanly (multipart uploads are aborted by minio on error). The
// returned stop function releases the signal handler.
func SignalContext(parent context.Context) (context.Context, context.CancelFunc) {
	return signal.NotifyContext(parent, os.Interrupt, syscall.SIGTERM)
}
