package objectstorage

import (
	"bytes"
	"io"
	"net/http"
	"regexp"
	"strings"

	sdk "github.com/latitudesh/latitudesh-go-sdk"
	"github.com/spf13/viper"
)

// NewAPIClient returns the Latitude SDK client used by the object storage
// commands. It is the regular authenticated client with one addition: a
// transport that normalizes known API/SDK mismatches in /storage responses
// before the generated models decode them.
//
// Today the API serializes `retention_period` as "" (or a numeric string) for
// buckets without object lock, while the v1.19.20 model declares it as
// *int64, which makes every bucket list/get fail with "cannot unmarshal string
// into Go value of type int64". Until the API returns null/number, the
// transport rewrites the field. Everything else passes through untouched.
func NewAPIClient() *sdk.Latitudesh {
	token := viper.GetString("Authorization")
	opts := []sdk.SDKOption{
		sdk.WithSecurity(token),
		sdk.WithClient(&normalizingHTTPClient{inner: http.DefaultClient}),
	}
	// Honour the global --hostname/--scheme overrides (dev/staging, tests)
	// the same way the legacy client does; the SDK default is production.
	if host := viper.GetString("hostname"); host != "" && host != "api.latitude.sh" {
		scheme := viper.GetString("scheme")
		if scheme == "" {
			scheme = "https"
		}
		opts = append(opts, sdk.WithServerURL(scheme+"://"+host))
	}
	return sdk.New(opts...)
}

type normalizingHTTPClient struct {
	inner *http.Client
}

// reRetentionString matches retention_period serialized as any JSON string.
var reRetentionString = regexp.MustCompile(`"retention_period"\s*:\s*"((?:[^"\\]|\\.)*)"`)

// Do executes the request and rewrites storage payloads.
func (c *normalizingHTTPClient) Do(req *http.Request) (*http.Response, error) {
	resp, err := c.inner.Do(req)
	if err != nil || resp == nil || resp.Body == nil {
		return resp, err
	}
	if !strings.Contains(req.URL.Path, "/storage/") || !strings.Contains(resp.Header.Get("Content-Type"), "json") {
		return resp, nil
	}
	body, readErr := io.ReadAll(resp.Body)
	resp.Body.Close()
	if readErr != nil {
		return nil, readErr
	}
	fixed := NormalizeStorageJSON(body)
	resp.Body = io.NopCloser(bytes.NewReader(fixed))
	resp.ContentLength = int64(len(fixed))
	resp.Header.Del("Content-Length")
	return resp, nil
}

// NormalizeStorageJSON applies the known field fixes to a /storage payload.
func NormalizeStorageJSON(body []byte) []byte {
	if !bytes.Contains(body, []byte(`"retention_period"`)) {
		return body
	}
	return reRetentionString.ReplaceAllFunc(body, func(m []byte) []byte {
		sub := reRetentionString.FindSubmatch(m)
		if len(sub) == 2 {
			if v := strings.TrimSpace(string(sub[1])); v != "" && isDigits(v) {
				return []byte(`"retention_period":` + v)
			}
		}
		return []byte(`"retention_period":null`)
	})
}

func isDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return s != ""
}
