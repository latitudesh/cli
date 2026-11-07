package elasticips

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	latitudeshgosdk "github.com/latitudesh/latitudesh-go-sdk"
	"github.com/latitudesh/latitudesh-go-sdk/models/operations"
	"github.com/latitudesh/lsh/internal/pagination"
)

// TestListElasticIPsWalksEveryPage serves 45 Elastic IPs in pages of
// page[size] like the API and checks that the list reads all of them, keeping
// the --server filter on every page. Before, only the first page (20 by
// default) was shown.
func TestListElasticIPsWalksEveryPage(t *testing.T) {
	const total = 45
	var pages, servers []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/elastic_ips" {
			http.NotFound(w, r)
			return
		}
		q := r.URL.Query()
		pages = append(pages, q.Get("page[number]"))
		servers = append(servers, q.Get("filter[server]"))
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
			items = append(items, fmt.Sprintf(`{"id":"eip_%d","type":"elastic_ips","attributes":{}}`, i))
		}
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprintf(w, `{"data":[%s]}`, strings.Join(items, ","))
	}))
	defer srv.Close()
	client := latitudeshgosdk.New(latitudeshgosdk.WithSecurity("test-token"), latitudeshgosdk.WithServerURL(srv.URL))

	server := "sv_1"
	got, result, err := listElasticIPs(context.Background(), client, operations.ListElasticIpsRequest{FilterServer: &server}, pagination.Options{PageSize: 20})
	if err != nil {
		t.Fatalf("listElasticIPs: %v", err)
	}
	if len(got.Data) != total {
		t.Errorf("got %d Elastic IPs, want %d", len(got.Data), total)
	}
	if p := strings.Join(pages, ","); p != "1,2,3" {
		t.Errorf("requested pages %s, want 1,2,3", p)
	}
	if s := strings.Join(servers, ","); s != "sv_1,sv_1,sv_1" {
		t.Errorf("filter[server] sent = %s, want sv_1 on every page", s)
	}
	if result.HasMore {
		t.Errorf("HasMore = true after the last page")
	}
}
