package s3

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	sdk "github.com/latitudesh/latitudesh-go-sdk"
	"github.com/latitudesh/lsh/internal/config"
	homedir "github.com/mitchellh/go-homedir"
)

// fakeAPI is a minimal Latitude API for the access-keys endpoints. It records
// the requests it receives and answers with canned JSON:API documents.
type fakeAPI struct {
	mu       sync.Mutex
	srv      *httptest.Server
	requests []fakeRequest
	// createShape is "wasabi" (access_key_id/secret_access_key) or "vast"
	// (access_key/secret_key).
	createShape string
	// keys is the GET /storage/access_keys document keyed by project.
	keys map[string]string
	// buckets is the GET /storage/buckets document.
	buckets string
	// deleteStatus is returned by DELETE (204 by default).
	deleteStatus int
}

type fakeRequest struct {
	Method string
	Path   string
	Query  url.Values
	Body   string
}

func newFakeAPI(t *testing.T) *fakeAPI {
	t.Helper()
	f := &fakeAPI{createShape: "wasabi", keys: map[string]string{}, deleteStatus: http.StatusNoContent}
	f.srv = httptest.NewServer(http.HandlerFunc(f.handle))
	t.Cleanup(f.srv.Close)
	return f
}

// client builds a keysAPI whose SDK client and raw GET both point at the fake
// server.
func (f *fakeAPI) client() *keysAPI {
	return &keysAPI{
		sdk:     sdk.New(sdk.WithServerURL(f.srv.URL), sdk.WithSecurity("test-token")),
		baseURL: f.srv.URL,
		token:   "test-token",
	}
}

func (f *fakeAPI) Requests() []fakeRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]fakeRequest(nil), f.requests...)
}

func (f *fakeAPI) handle(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	f.mu.Lock()
	f.requests = append(f.requests, fakeRequest{Method: r.Method, Path: r.URL.Path, Query: r.URL.Query(), Body: string(body)})
	f.mu.Unlock()
	w.Header().Set("Content-Type", "application/vnd.api+json")

	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/storage/access_keys":
		var req struct {
			Data struct {
				Attributes struct {
					Name string `json:"name"`
				} `json:"attributes"`
			} `json:"data"`
		}
		_ = json.Unmarshal(body, &req)
		name := strings.ToLower(req.Data.Attributes.Name)
		w.WriteHeader(http.StatusCreated)
		if f.createShape == "vast" {
			io.WriteString(w, `{"data":{"type":"access_keys","attributes":{"access_key":{"access_key":"VASTKEYID","secret_key":"vast-secret-value","name":"`+name+`","status":"Active","username":"user_1-`+name+`"}}}}`)
			return
		}
		io.WriteString(w, `{"data":{"type":"access_keys","attributes":{"access_key":{"access_key_id":"WASABIKEYID","secret_access_key":"wasabi-secret-value","name":"`+name+`","status":"Active","username":"someone+`+name+`@example.com"}}}}`)
	case r.Method == http.MethodGet && r.URL.Path == "/storage/access_keys":
		doc, ok := f.keys[r.URL.Query().Get("project")]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			io.WriteString(w, `{"errors":[{"status":"404","title":"Not Found","detail":"project not found"}]}`)
			return
		}
		io.WriteString(w, doc)
	case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/storage/access_keys/"):
		w.WriteHeader(f.deleteStatus)
		if f.deleteStatus == http.StatusNotFound {
			io.WriteString(w, `{"errors":[{"status":"404","title":"Not Found","detail":"access key not found"}]}`)
		}
	case r.Method == http.MethodGet && r.URL.Path == "/storage/buckets":
		if f.buckets == "" {
			io.WriteString(w, `{"data":[]}`)
			return
		}
		io.WriteString(w, f.buckets)
	default:
		w.WriteHeader(http.StatusNotFound)
		io.WriteString(w, `{"errors":[{"status":"404","title":"Not Found"}]}`)
	}
}

// keysDocument is the canned GET /storage/access_keys payload: one standard
// fullaccess key, one standard limited key and one high_performance key,
// including the buckets[]/access fields the SDK drops.
const keysDocument = `{"data":{
  "standard":[
    {"name":"ops","username":"ops@example.com","access_key_id":"OPSKEYID000000000001","status":"Active","created_at":"2026-09-01T10:00:00Z","region":"DAL","access":"fullaccess","buckets":["backups-7f3a","logs-91aa"]},
    {"name":"ci-deploy","username":"ci@example.com","access_key_id":"CIKEYID0000000000002","status":"Active","created_at":"2026-09-02T10:00:00Z","region":"DAL","access":"rw","buckets":["backups-7f3a"]}
  ],
  "high_performance":[
    {"name":"fast","username":"user_1-fast","access_key_id":"HPKEYID0000000000003","status":"Active","created_at":"2026-09-03T10:00:00Z","region":"TYO4","access":"readonly","buckets":["fast-bucket-01"]}
  ]}}`

// withTempProfile points the config file at a temporary HOME holding one
// profile named "test" with the given saved keys, and returns the directory.
func withTempProfile(t *testing.T, keys map[string]config.StoredAccessKey) string {
	t.Helper()
	dir := t.TempDir()
	homedir.DisableCache = true
	t.Setenv("HOME", dir)
	t.Setenv("LSH_PROFILE", "")
	t.Setenv("LATITUDESH_TOKEN", "")
	f := &config.File{
		DefaultProfile: "test",
		Profiles: map[string]config.Profile{
			"test": {Authorization: "tok", Email: "Lanusse.Morais@latitude.sh", ObjectStorage: &config.ObjectStorageConfig{Keys: keys}},
		},
	}
	if err := config.Save(f); err != nil {
		t.Fatalf("save config: %v", err)
	}
	return dir
}

// readConfigRaw returns the on-disk config so tests can assert on it.
func readConfigRaw(t *testing.T, home string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(home, ".config", "lsh", "config.json"))
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	return string(b)
}
