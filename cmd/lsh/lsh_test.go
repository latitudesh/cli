package lsh

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

// recordingTransport answers every request locally and remembers its URL, so
// the tests can see where the SDK client sends requests without any network.
type recordingTransport struct {
	urls []string
}

func (rt *recordingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	rt.urls = append(rt.urls, req.URL.String())
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/vnd.api+json"}},
		Body:       io.NopCloser(strings.NewReader(`{"data":[]}`)),
		Request:    req,
	}, nil
}

func setAPIHostSettings(t *testing.T, hostname, scheme, basePath string) {
	t.Helper()
	viper.Reset()
	t.Cleanup(viper.Reset)
	viper.Set("Authorization", "test-token")
	if hostname != "" {
		viper.Set("hostname", hostname)
	}
	if scheme != "" {
		viper.Set("scheme", scheme)
	}
	if basePath != "" {
		viper.Set("base_path", basePath)
	}
}

func TestServerURL(t *testing.T) {
	cases := []struct {
		name                       string
		hostname, scheme, basePath string
		want                       string
	}{
		{name: "nothing set keeps the SDK default", want: ""},
		{name: "flag defaults keep the SDK default", hostname: "api.latitude.sh", scheme: "https", basePath: "/", want: ""},
		{name: "hostname override", hostname: "api.staging.example.com", want: "https://api.staging.example.com"},
		{name: "scheme override", scheme: "http", want: "http://api.latitude.sh"},
		{name: "host with port and scheme", hostname: "localhost:8080", scheme: "http", basePath: "/", want: "http://localhost:8080"},
		{name: "base path", hostname: "api.staging.example.com", basePath: "/v2/", want: "https://api.staging.example.com/v2"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setAPIHostSettings(t, tc.hostname, tc.scheme, tc.basePath)
			if got := ServerURL(); got != tc.want {
				t.Errorf("ServerURL() = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestNewClientHonorsAPIHostSettings checks that the SDK client used by the
// SDK-backed commands sends requests to the host chosen with --hostname,
// --scheme and --base-path, as the legacy client and `lsh login` do. Before,
// it always used https://api.latitude.sh, so `lsh --hostname <staging> ...`
// sent the staging token to production.
func TestNewClientHonorsAPIHostSettings(t *testing.T) {
	cases := []struct {
		name                       string
		hostname, scheme, basePath string
		want                       string
	}{
		{name: "default", want: "https://api.latitude.sh/tags"},
		{name: "hostname and scheme", hostname: "api.staging.example.com", scheme: "http", want: "http://api.staging.example.com/tags"},
		{name: "base path", hostname: "api.staging.example.com", basePath: "/v2", want: "https://api.staging.example.com/v2/tags"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setAPIHostSettings(t, tc.hostname, tc.scheme, tc.basePath)
			rt := &recordingTransport{}
			orig := http.DefaultTransport
			http.DefaultTransport = rt
			t.Cleanup(func() { http.DefaultTransport = orig })

			if _, err := NewClient().Tags.List(context.Background()); err != nil {
				t.Fatalf("Tags.List: %v", err)
			}
			if len(rt.urls) != 1 || rt.urls[0] != tc.want {
				t.Errorf("requests = %q, want [%q]", rt.urls, tc.want)
			}
		})
	}
}
