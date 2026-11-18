package cli

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/spf13/viper"
)

// plans list/stock talk to the API by hand instead of through the SDK, so
// they must build their URL from the same root flags the SDK client uses.
func TestFetchPlansUsesConfiguredServerURL(t *testing.T) {
	var gotPath, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer srv.Close()

	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}

	viper.Reset()
	t.Cleanup(viper.Reset)
	viper.Set("scheme", u.Scheme)
	viper.Set("hostname", u.Host)
	viper.Set("base_path", "/")

	if _, err := fetchPlans(context.Background(), "test-token"); err != nil {
		t.Fatalf("fetchPlans against the test server: %v", err)
	}
	if gotPath != "/plans" {
		t.Fatalf("request path = %q, want /plans", gotPath)
	}
	if gotAuth != "test-token" {
		t.Fatalf("Authorization header = %q, want test-token", gotAuth)
	}
}
