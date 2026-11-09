package cli

import (
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

type regionsAPI struct{ requests int }

func (rt *regionsAPI) RoundTrip(req *http.Request) (*http.Response, error) {
	rt.requests++
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/vnd.api+json"}},
		Body: io.NopCloser(strings.NewReader(`{"data":[
			{"id":"loc_1","type":"regions","attributes":{"slug":"SAO","name":"Sao Paulo"}},
			{"id":"loc_2","type":"regions","attributes":{"slug":"DAL","name":"Dallas"}}]}`)),
		Request: req,
	}, nil
}

// With --no-paginate, a first page shorter than --page-size is all there is,
// so the "More results available" hint must not be printed.
func TestRegionsListNoPaginateShortPageNoNextHint(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	viper.Set("Authorization", "test-token")
	viper.Set("page-size", 100)
	viper.Set("no-paginate", true)
	viper.Set("output", "json")

	api := &regionsAPI{}
	orig := http.DefaultTransport
	http.DefaultTransport = api
	t.Cleanup(func() { http.DefaultTransport = orig })

	stderr := captureStderr(t, func() {
		if err := runRegionsList(nil, nil); err != nil {
			t.Fatalf("runRegionsList: %v", err)
		}
	})

	if api.requests != 1 {
		t.Errorf("expected one request, got %d", api.requests)
	}
	if strings.Contains(stderr, "More results available") {
		t.Errorf("printed a next-page hint for a short last page:\n%s", stderr)
	}
}

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	orig := os.Stderr
	os.Stderr = w
	defer func() { os.Stderr = orig }()
	fn()
	w.Close()
	out, _ := io.ReadAll(r)
	return string(out)
}
