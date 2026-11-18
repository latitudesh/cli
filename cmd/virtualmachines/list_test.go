package virtualmachines

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	latitudeshgosdk "github.com/latitudesh/latitudesh-go-sdk"
	"github.com/latitudesh/lsh/internal/pagination"
)

// fakeVMAPI serves GET /virtual_machines with pages of `pageSize` items up to
// `total` items, mirroring how the live API paginates, and records every
// request's query string so tests can assert what the SDK sent.
func fakeVMAPI(t *testing.T, total, pageSize int) (*latitudeshgosdk.Latitudesh, *[]url.Values) {
	t.Helper()
	var requests []url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/virtual_machines" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		q := r.URL.Query()
		requests = append(requests, q)

		page := 1
		fmt.Sscanf(q.Get("page[number]"), "%d", &page)
		start := (page - 1) * pageSize
		var items []string
		for i := start; i < start+pageSize && i < total; i++ {
			items = append(items, fmt.Sprintf(`{"id":"vm_%d","type":"virtual_machines","attributes":{"name":"vm-%d"}}`, i+1, i+1))
		}
		w.Header().Set("Content-Type", "application/vnd.api+json")
		fmt.Fprintf(w, `{"data":[%s]}`, strings.Join(items, ","))
	}))
	t.Cleanup(srv.Close)

	client := latitudeshgosdk.New(
		latitudeshgosdk.WithSecurity("test-token"),
		latitudeshgosdk.WithServerURL(srv.URL),
	)
	return client, &requests
}

func vmIDs(vms VirtualMachines) []string {
	ids := make([]string, 0, len(vms.Data))
	for _, vm := range vms.Data {
		ids = append(ids, getStr(vm.ID))
	}
	return ids
}

func TestListVirtualMachinesWalksEveryPageWithFilters(t *testing.T) {
	client, requests := fakeVMAPI(t, 5, 2)
	project, tags := "my-project", "tag_1,tag_2"

	vms, result, err := listVirtualMachines(context.Background(), client, &project, &tags, pagination.Options{PageSize: 2})
	if err != nil {
		t.Fatal(err)
	}

	if got := vmIDs(vms); strings.Join(got, ",") != "vm_1,vm_2,vm_3,vm_4,vm_5" {
		t.Fatalf("collected %v, want every item across the three pages", got)
	}
	if result.HasMore {
		t.Fatal("a short last page must end the walk")
	}
	// Three pages: two full ones and a short one that ends the walk.
	if len(*requests) != 3 {
		t.Fatalf("made %d requests, want 3", len(*requests))
	}
	for i, q := range *requests {
		if q.Get("page[number]") != fmt.Sprint(i+1) || q.Get("page[size]") != "2" {
			t.Errorf("request %d paging = %v, want page[number]=%d page[size]=2", i+1, q, i+1)
		}
		// The SDK's Next closure must carry the filters to every later page.
		if q.Get("filter[project]") != project || q.Get("filter[tags]") != tags {
			t.Errorf("request %d dropped the filters: %v", i+1, q)
		}
	}
}

func TestListVirtualMachinesHonorsMaxItems(t *testing.T) {
	client, requests := fakeVMAPI(t, 10, 2)

	vms, _, err := listVirtualMachines(context.Background(), client, nil, nil, pagination.Options{PageSize: 2, MaxItems: 3})
	if err != nil {
		t.Fatal(err)
	}

	if got := vmIDs(vms); strings.Join(got, ",") != "vm_1,vm_2,vm_3" {
		t.Fatalf("collected %v, want the first 3 items", got)
	}
	// Two pages reach the cap; the walk must not fetch a third.
	if len(*requests) != 2 {
		t.Fatalf("made %d requests, want 2", len(*requests))
	}
}

func TestListVirtualMachinesNoPaginateFetchesOnePage(t *testing.T) {
	client, requests := fakeVMAPI(t, 10, 2)

	vms, result, err := listVirtualMachines(context.Background(), client, nil, nil, pagination.Options{PageSize: 2, NoPaginate: true})
	if err != nil {
		t.Fatal(err)
	}

	if got := vmIDs(vms); strings.Join(got, ",") != "vm_1,vm_2" {
		t.Fatalf("collected %v, want only the first page", got)
	}
	if len(*requests) != 1 {
		t.Fatalf("made %d requests, want 1", len(*requests))
	}
	if !result.HasMore || result.NextPage != 2 {
		t.Fatalf("result = %+v, want HasMore with NextPage 2 so the cursor hint is printed", result)
	}
}
