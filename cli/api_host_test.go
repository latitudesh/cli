package cli

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// hostRecorder answers every request locally and remembers its URL.
type hostRecorder struct {
	urls []string
}

func (rt *hostRecorder) RoundTrip(req *http.Request) (*http.Response, error) {
	rt.urls = append(rt.urls, req.URL.String())
	body := `{"data":[]}`
	if strings.HasPrefix(req.URL.Path, "/team/") {
		body = `{"data":{"id":"tm_1","type":"teams","attributes":{"name":"New"}}}`
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/vnd.api+json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    req,
	}, nil
}

func useStagingHost(t *testing.T) *hostRecorder {
	t.Helper()
	withTempHome(t)
	viper.Reset()
	t.Cleanup(viper.Reset)
	viper.Set("Authorization", "test-token")
	viper.Set("hostname", "api.staging.example.com")
	viper.Set("scheme", "http")

	rt := &hostRecorder{}
	orig := http.DefaultTransport
	http.DefaultTransport = rt
	t.Cleanup(func() { http.DefaultTransport = orig })
	return rt
}

func assertStagingHost(t *testing.T, rt *hostRecorder, wantPath string) {
	t.Helper()
	want := "http://api.staging.example.com" + wantPath
	if len(rt.urls) != 1 || !strings.HasPrefix(rt.urls[0], want) {
		t.Errorf("requests = %q, want one request to %q", rt.urls, want)
	}
}

// The commands below build their own SDK client instead of using the shared
// one, so they have to pick up --hostname/--scheme as well.

func TestTeamsUpdateHonorsAPIHostSettings(t *testing.T) {
	rt := useStagingHost(t)

	cmd := &cobra.Command{}
	cmd.Flags().String("name", "", "")
	cmd.Flags().String("address", "", "")
	cmd.Flags().String("profile", "", "")
	if err := cmd.Flags().Set("name", "New"); err != nil {
		t.Fatal(err)
	}

	if err := runTeamsUpdate(cmd, []string{"tm_1"}); err != nil {
		t.Fatalf("runTeamsUpdate: %v", err)
	}
	assertStagingHost(t, rt, "/team/tm_1")
}

func TestVolumeListHonorsAPIHostSettings(t *testing.T) {
	rt := useStagingHost(t)

	op := &VolumeListOperation{}
	cmd, err := op.Register()
	if err != nil {
		t.Fatal(err)
	}
	if err := op.run(cmd, nil); err != nil {
		t.Fatalf("volume list: %v", err)
	}
	assertStagingHost(t, rt, "/storage/volumes")
}
