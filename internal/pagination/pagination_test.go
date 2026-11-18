package pagination

import (
	"testing"

	"github.com/spf13/viper"
)

func resetPaginationViper() {
	viper.Set("page-size", int64(0))
	viper.Set("max-items", int64(0))
	viper.Set("no-paginate", false)
}

func TestValidateRejectsNonPositivePageSize(t *testing.T) {
	defer resetPaginationViper()

	resetPaginationViper()
	viper.Set("page-size", int64(0))
	if err := Validate(); err == nil {
		t.Fatal("expected error for --page-size 0")
	}

	resetPaginationViper()
	viper.Set("page-size", int64(-5))
	if err := Validate(); err == nil {
		t.Fatal("expected error for negative --page-size")
	}

	resetPaginationViper()
	viper.Set("page-size", int64(100))
	if err := Validate(); err != nil {
		t.Fatalf("unexpected error for valid --page-size: %v", err)
	}
}

func TestValidateRejectsNegativeMaxItems(t *testing.T) {
	defer resetPaginationViper()

	resetPaginationViper()
	viper.Set("page-size", int64(100))
	viper.Set("max-items", int64(-1))
	if err := Validate(); err == nil {
		t.Fatal("expected error for negative --max-items")
	}

	resetPaginationViper()
	viper.Set("page-size", int64(100))
	viper.Set("max-items", int64(0)) // 0 = unlimited, valid.
	if err := Validate(); err != nil {
		t.Fatalf("unexpected error for --max-items=0 (unlimited): %v", err)
	}
}

// fakePage models an SDK list response: a slice of items plus a closure that
// returns the next page (nil when exhausted).
type fakePage struct {
	items []int
	next  func() (*fakePage, error)
}

// makePages builds a chain of pages of the given size from a flat list of
// items, recording how many times a page is actually fetched. The last page
// carries no next closure.
func makePages(items []int, pageSize int, calls *int) *fakePage {
	var build func(start int) *fakePage
	build = func(start int) *fakePage {
		end := start + pageSize
		if end > len(items) {
			end = len(items)
		}
		p := &fakePage{items: items[start:end]}
		if end < len(items) {
			p.next = func() (*fakePage, error) {
				*calls++
				return build(end), nil
			}
		}
		return p
	}
	return build(0)
}

// sdkStylePages mirrors the generated SDK: every response carries a non-nil
// Next closure, which returns (nil, nil) once a page comes back empty or
// shorter than pageSize, and otherwise fetches the following page.
func sdkStylePages(items []int, pageSize int, calls *int) *fakePage {
	var build func(start int) *fakePage
	build = func(start int) *fakePage {
		end := start + pageSize
		if end > len(items) {
			end = len(items)
		}
		p := &fakePage{items: items[start:end]}
		p.next = func() (*fakePage, error) {
			*calls++
			if len(p.items) == 0 || len(p.items) < pageSize {
				return nil, nil
			}
			return build(end), nil
		}
		return p
	}
	return build(0)
}

func collect(first *fakePage, opts Options) ([]int, Result, error) {
	return collectWhere(first, opts, func(int) bool { return true })
}

// collectWhere walks the pages keeping only the items that pass keep, the way
// `servers list --operating_system` filters rows client-side. Once the cap is
// reached it keeps scanning so Unread counts only the matches left behind.
func collectWhere(first *fakePage, opts Options, keep func(int) bool) ([]int, Result, error) {
	var got []int
	res, err := Walk(first, opts,
		func(p *fakePage) func() (*fakePage, error) { return p.next },
		func(p *fakePage, limit int) PageInfo {
			info := PageInfo{Len: len(p.items)}
			for _, item := range p.items {
				if !keep(item) {
					continue
				}
				if limit >= 0 && info.Added >= limit {
					info.Unread++
					continue
				}
				got = append(got, item)
				info.Added++
			}
			return info
		},
	)
	return got, res, err
}

func seq(n int) []int {
	items := make([]int, n)
	for i := range items {
		items[i] = i + 1
	}
	return items
}

func TestWalkFetchesAllPagesByDefault(t *testing.T) {
	items := []int{1, 2, 3, 4, 5, 6, 7}
	calls := 0
	first := makePages(items, 3, &calls)

	got, res, err := collect(first, Options{PageSize: 3})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != len(items) {
		t.Fatalf("expected %d items, got %d", len(items), len(got))
	}
	if res.HasMore {
		t.Fatalf("expected HasMore=false after exhausting pages")
	}
	// 7 items / 3 per page = pages at offset 0,3,6 → 2 follow-up fetches.
	if calls != 2 {
		t.Fatalf("expected 2 follow-up fetches, got %d", calls)
	}
}

func TestWalkMaxItemsStopsEarly(t *testing.T) {
	// 100 items, page size 10, max 50 → at most 5 pages (4 follow-up calls).
	calls := 0
	first := makePages(seq(100), 10, &calls)

	got, res, err := collect(first, Options{PageSize: 10, MaxItems: 50})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 50 {
		t.Fatalf("expected 50 items, got %d", len(got))
	}
	if calls != 4 {
		t.Fatalf("expected 4 follow-up fetches (5 pages total), got %d", calls)
	}
	if !res.HasMore {
		t.Fatalf("expected HasMore=true (more pages remain)")
	}
}

func TestWalkMaxItemsTrimsFinalPage(t *testing.T) {
	calls := 0
	first := makePages(seq(100), 10, &calls)

	// 45 is not a multiple of the page size: the 5th page must be trimmed to 5.
	got, _, err := collect(first, Options{PageSize: 10, MaxItems: 45})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 45 {
		t.Fatalf("expected 45 items, got %d", len(got))
	}
}

func TestWalkNoPaginateReturnsFirstPageOnly(t *testing.T) {
	items := []int{1, 2, 3, 4, 5}
	calls := 0
	first := makePages(items, 2, &calls)

	got, res, err := collect(first, Options{PageSize: 2, NoPaginate: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 items (first page), got %d", len(got))
	}
	if calls != 0 {
		t.Fatalf("expected no follow-up fetches, got %d", calls)
	}
	if !res.HasMore || res.NextPage != 2 {
		t.Fatalf("expected HasMore=true and NextPage=2, got %+v", res)
	}
}

func TestWalkNextReturnsNilResponse(t *testing.T) {
	// A page whose Next closure is non-nil but returns (nil, nil) to signal the
	// end — must not be treated as a real page (and must not panic).
	last := &fakePage{items: []int{3, 4}}
	first := &fakePage{
		items: []int{1, 2},
		next:  func() (*fakePage, error) { return last, nil },
	}
	last.next = func() (*fakePage, error) { return nil, nil }

	got, res, err := collect(first, Options{PageSize: 2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 4 {
		t.Fatalf("expected 4 items, got %d (%v)", len(got), got)
	}
	if res.HasMore {
		t.Fatalf("expected HasMore=false")
	}
}

func TestWalkNilFirstPage(t *testing.T) {
	got, res, err := collect(nil, Options{PageSize: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 || res.HasMore {
		t.Fatalf("expected empty result for nil first page, got %v %+v", got, res)
	}
}

// The SDK sets Next on every response, so Walk cannot use its presence to
// decide whether another page exists when it stops early. The cases below
// cover the early-exit branches with SDK-style pages.

func TestWalkNoPaginateShortPageHasNoMore(t *testing.T) {
	calls := 0
	first := sdkStylePages([]int{1, 2, 3}, 100, &calls)

	got, res, err := collect(first, Options{PageSize: 100, NoPaginate: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 3 || calls != 0 {
		t.Fatalf("expected 3 items and no follow-up fetch, got %d items, %d calls", len(got), calls)
	}
	if res.HasMore {
		t.Fatalf("expected HasMore=false for a page shorter than page[size], got %+v", res)
	}
}

func TestWalkNoPaginateFullPageHasMore(t *testing.T) {
	calls := 0
	first := sdkStylePages([]int{1, 2, 3, 4, 5}, 2, &calls)

	_, res, err := collect(first, Options{PageSize: 2, NoPaginate: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 0 {
		t.Fatalf("expected no follow-up fetch, got %d", calls)
	}
	if !res.HasMore || res.NextPage != 2 {
		t.Fatalf("expected HasMore=true and NextPage=2, got %+v", res)
	}
}

func TestWalkMaxItemsTrimmedPageHasMore(t *testing.T) {
	calls := 0
	first := sdkStylePages([]int{1, 2, 3, 4, 5}, 100, &calls)

	// The cap trims the page, so items are left even though the page is short.
	got, res, err := collect(first, Options{PageSize: 100, MaxItems: 2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 items, got %d", len(got))
	}
	if !res.HasMore {
		t.Fatalf("expected HasMore=true when --max-items trimmed the page, got %+v", res)
	}
}

func TestWalkMaxItemsExactCapOnShortPageHasNoMore(t *testing.T) {
	calls := 0
	first := sdkStylePages([]int{1, 2, 3}, 100, &calls)

	// The cap equals the page length: nothing was trimmed and the page is
	// short, so there is nothing left to fetch.
	got, res, err := collect(first, Options{PageSize: 100, MaxItems: 3})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 3 || calls != 0 {
		t.Fatalf("expected 3 items and no follow-up fetch, got %d items, %d calls", len(got), calls)
	}
	if res.HasMore {
		t.Fatalf("expected HasMore=false when the cap matches a short last page, got %+v", res)
	}
}

func TestWalkMaxItemsExactCapOnFullPageHasMore(t *testing.T) {
	calls := 0
	first := sdkStylePages(seq(10), 5, &calls)

	// The cap equals a full page: later pages may still hold items.
	_, res, err := collect(first, Options{PageSize: 5, MaxItems: 5})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.HasMore {
		t.Fatalf("expected HasMore=true when the cap matches a full page, got %+v", res)
	}
}

func TestWalkNoPaginateFilteredFullPageHasMore(t *testing.T) {
	calls := 0
	first := sdkStylePages(seq(100), 100, &calls)

	// A client-side filter keeps 2 of the 100 rows: the page is still full, so
	// later pages may hold more matches and the hint must stay.
	got, res, err := collectWhere(first, Options{PageSize: 100, NoPaginate: true},
		func(item int) bool { return item%50 == 0 })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 || calls != 0 {
		t.Fatalf("expected 2 items and no follow-up fetch, got %d items, %d calls", len(got), calls)
	}
	if !res.HasMore {
		t.Fatalf("expected HasMore=true for a full page filtered client-side, got %+v", res)
	}
}

func TestWalkNoPaginateFilteredShortPageHasNoMore(t *testing.T) {
	calls := 0
	first := sdkStylePages(seq(40), 100, &calls)

	got, res, err := collectWhere(first, Options{PageSize: 100, NoPaginate: true},
		func(item int) bool { return item%10 == 0 })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 4 {
		t.Fatalf("expected 4 items, got %d", len(got))
	}
	if res.HasMore {
		t.Fatalf("expected HasMore=false for a short page filtered client-side, got %+v", res)
	}
}

func TestWalkMaxItemsFilteredExactCapOnShortPageHasNoMore(t *testing.T) {
	calls := 0
	first := sdkStylePages([]int{1, 2}, 100, &calls)

	// The only match is the last row of a short page and the cap equals the
	// match count: the filtered-out row must not count as left behind.
	got, res, err := collectWhere(first, Options{PageSize: 100, MaxItems: 1, NoPaginate: true},
		func(item int) bool { return item%2 == 0 })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || calls != 0 {
		t.Fatalf("expected 1 item and no follow-up fetch, got %d items, %d calls", len(got), calls)
	}
	if res.HasMore {
		t.Fatalf("expected HasMore=false when the cap matches the filtered rows of a short page, got %+v", res)
	}
}

func TestWalkMaxItemsFilteredCapWithMatchesLeftHasMore(t *testing.T) {
	calls := 0
	first := sdkStylePages([]int{2, 4, 6}, 100, &calls)

	// Two matches remain after the cap on a short page: they were left behind.
	got, res, err := collectWhere(first, Options{PageSize: 100, MaxItems: 1},
		func(item int) bool { return item%2 == 0 })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 item, got %d", len(got))
	}
	if !res.HasMore {
		t.Fatalf("expected HasMore=true when matches were left behind by the cap, got %+v", res)
	}
}

func TestWalkMaxItemsFilteredKeepsFetchingUntilCap(t *testing.T) {
	calls := 0
	first := sdkStylePages(seq(100), 10, &calls)

	// 3 matches per page of 10; the cap of 7 needs three pages and trims the
	// third, so the result must say more is available.
	got, res, err := collectWhere(first, Options{PageSize: 10, MaxItems: 7},
		func(item int) bool { return item%10 < 3 })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 7 || calls != 2 {
		t.Fatalf("expected 7 items from 3 pages, got %d items, %d follow-up fetches", len(got), calls)
	}
	if !res.HasMore {
		t.Fatalf("expected HasMore=true when the cap trimmed a filtered page, got %+v", res)
	}
}
