package cmd

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/latitudesh/lsh/cli"
	"github.com/spf13/viper"
)

type cannedAPI struct {
	status int
	body   string
}

func (c cannedAPI) RoundTrip(req *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: c.status,
		Header:     http.Header{"Content-Type": []string{"application/vnd.api+json"}},
		Body:       io.NopCloser(strings.NewReader(c.body)),
		Request:    req,
	}, nil
}

var (
	testRootOnce sync.Once
	testRootErr  error
)

func testRoot(t *testing.T) {
	t.Helper()
	testRootOnce.Do(func() {
		_, testRootErr = cli.MakeRootCmd(rootCmd)
	})
	if testRootErr != nil {
		t.Fatalf("MakeRootCmd: %v", testRootErr)
	}
}

// Create commands print a confirmation banner next to the created resource.
// With -o json the banner must stay off stdout, otherwise `lsh ... -o json | jq`
// fails on the first line.
func TestCreateCommandsJSONOutputIsValid(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("LATITUDESH_TOKEN", "test-token")
	testRoot(t)

	cases := []struct {
		name   string
		args   []string
		status int
		body   string
	}{
		{
			name:   "firewalls create",
			args:   []string{"firewalls", "create", "--name", "web", "--project", "proj_x"},
			status: http.StatusCreated,
			body:   `{"data":{"id":"fw_1","type":"firewalls","attributes":{"name":"web","rules":[]}}}`,
		},
		{
			name:   "firewalls assignments create",
			args:   []string{"firewalls", "assignments", "create", "--firewall", "fw_1", "--server", "sv_1"},
			status: http.StatusCreated,
			body:   `{"data":{"id":"fwasg_1","type":"firewall_assignments","attributes":{"firewall_id":"fw_1","server":{"id":"sv_1"}}}}`,
		},
		{
			name:   "storage-objects create",
			args:   []string{"storage-objects", "create", "--project", "proj_x", "--name", "logs", "--region", "SAO2"},
			status: http.StatusCreated,
			body:   `{"data":{"id":"st_1","type":"object_storages","attributes":{"name":"logs","region":{"id":"SAO2","city":"Sao Paulo"}}}}`,
		},
		{
			name:   "elastic-ips create",
			args:   []string{"elastic-ips", "create", "--project", "proj_x", "--server", "sv_1"},
			status: http.StatusAccepted,
			body:   `{"data":{"id":"eip_1","type":"elastic_ips","attributes":{"address":"203.0.113.10","status":"configuring"}}}`,
		},
		{
			name:   "tags create",
			args:   []string{"tags", "create", "--no-input", "--name", "prod", "--color", "#FF0000"},
			status: http.StatusCreated,
			body:   `{"data":{"id":"tag_1","type":"tags","attributes":{"name":"prod","color":"#FF0000"}}}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			viper.Set("output", "json")
			t.Cleanup(func() { viper.Set("output", "table") })

			orig := http.DefaultTransport
			http.DefaultTransport = cannedAPI{status: tc.status, body: tc.body}
			t.Cleanup(func() { http.DefaultTransport = orig })

			rootCmd.SetArgs(append(tc.args, "-o", "json"))
			var execErr error
			stdout := captureStdout(t, func() { _, execErr = rootCmd.ExecuteC() })
			if execErr != nil {
				t.Fatalf("execute: %v", execErr)
			}

			if !json.Valid([]byte(stdout)) {
				t.Fatalf("stdout is not valid JSON:\n%s", stdout)
			}
		})
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	orig := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = orig }()

	done := make(chan string)
	go func() {
		out, _ := io.ReadAll(r)
		done <- string(out)
	}()
	fn()
	_ = w.Close()
	return <-done
}
