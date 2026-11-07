package firewalls

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	latitudeshgosdk "github.com/latitudesh/latitudesh-go-sdk"
	"github.com/latitudesh/lsh/internal/pagination"
)

// pagedAPI serves `total` JSON:API items of the given type on path, split
// into pages by page[size] and page[number] like the Latitude.sh API, and
// records the page numbers it was asked for.
type pagedAPI struct {
	mu    sync.Mutex
	pages []string
	sizes []string
}

func newPagedAPI(t *testing.T, path, itemType, idPrefix string, total int) (*pagedAPI, *latitudeshgosdk.Latitudesh) {
	t.Helper()
	api := &pagedAPI{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != path {
			http.NotFound(w, r)
			return
		}
		q := r.URL.Query()
		api.mu.Lock()
		api.pages = append(api.pages, q.Get("page[number]"))
		api.sizes = append(api.sizes, q.Get("page[size]"))
		api.mu.Unlock()

		size, _ := strconv.Atoi(q.Get("page[size]"))
		if size <= 0 {
			size = 20
		}
		number, _ := strconv.Atoi(q.Get("page[number]"))
		if number <= 0 {
			number = 1
		}
		var items []string
		for i := (number-1)*size + 1; i <= total && i <= number*size; i++ {
			items = append(items, fmt.Sprintf(`{"id":"%s%d","type":"%s","attributes":{}}`, idPrefix, i, itemType))
		}
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprintf(w, `{"data":[%s]}`, strings.Join(items, ","))
	}))
	t.Cleanup(srv.Close)
	client := latitudeshgosdk.New(
		latitudeshgosdk.WithSecurity("test-token"),
		latitudeshgosdk.WithServerURL(srv.URL),
	)
	return api, client
}

func TestListFirewallsWalksEveryPage(t *testing.T) {
	cases := []struct {
		name      string
		opts      pagination.Options
		wantIDs   int
		wantPages string
		wantMore  bool
	}{
		{name: "all pages", opts: pagination.Options{PageSize: 2}, wantIDs: 5, wantPages: "1,2,3"},
		{name: "max items", opts: pagination.Options{PageSize: 2, MaxItems: 3}, wantIDs: 3, wantPages: "1,2", wantMore: true},
		{name: "no paginate", opts: pagination.Options{PageSize: 2, NoPaginate: true}, wantIDs: 2, wantPages: "1", wantMore: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			api, client := newPagedAPI(t, "/firewalls", "firewalls", "fw_", 5)
			got, result, err := listFirewalls(context.Background(), client, nil, tc.opts)
			if err != nil {
				t.Fatalf("listFirewalls: %v", err)
			}
			if len(got.Data) != tc.wantIDs {
				t.Errorf("got %d firewalls, want %d", len(got.Data), tc.wantIDs)
			}
			if p := strings.Join(api.pages, ","); p != tc.wantPages {
				t.Errorf("requested pages %s, want %s", p, tc.wantPages)
			}
			for _, size := range api.sizes {
				if size != "2" {
					t.Errorf("page[size] sent = %q, want 2 on every request", api.sizes)
					break
				}
			}
			if result.HasMore != tc.wantMore {
				t.Errorf("HasMore = %v, want %v", result.HasMore, tc.wantMore)
			}
		})
	}
}

func TestListAssignmentsWalksEveryPage(t *testing.T) {
	t.Run("team assignments", func(t *testing.T) {
		api, client := newPagedAPI(t, "/firewalls/assignments", "firewall_assignments", "fwasg_", 45)
		got, _, err := listAssignments(context.Background(), client, "", nil, pagination.Options{PageSize: 20})
		if err != nil {
			t.Fatalf("listAssignments: %v", err)
		}
		if len(got.Data) != 45 {
			t.Errorf("got %d assignments, want 45", len(got.Data))
		}
		if p := strings.Join(api.pages, ","); p != "1,2,3" {
			t.Errorf("requested pages %s, want 1,2,3", p)
		}
	})
	t.Run("one firewall", func(t *testing.T) {
		api, client := newPagedAPI(t, "/firewalls/fw_1/assignments", "firewall_assignments", "fwasg_", 45)
		got, _, err := listAssignments(context.Background(), client, "fw_1", nil, pagination.Options{PageSize: 20})
		if err != nil {
			t.Fatalf("listAssignments: %v", err)
		}
		if len(got.Data) != 45 {
			t.Errorf("got %d assignments, want 45", len(got.Data))
		}
		if p := strings.Join(api.pages, ","); p != "1,2,3" {
			t.Errorf("requested pages %s, want 1,2,3", p)
		}
	})
}
