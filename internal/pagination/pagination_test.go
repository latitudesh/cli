package pagination

import "testing"

// fakePage models an SDK list response: a slice of items plus a closure that
// returns the next page (nil when exhausted).
type fakePage struct {
	items []int
	next  func() (*fakePage, error)
}

// makePages builds a chain of pages of the given size from a flat list of
// items, recording how many times a page is actually fetched.
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

func collect(first *fakePage, opts Options) ([]int, Result, error) {
	var got []int
	res, err := Walk(first, opts,
		func(p *fakePage) func() (*fakePage, error) { return p.next },
		func(p *fakePage, limit int) int {
			n := len(p.items)
			if limit >= 0 && n > limit {
				n = limit
			}
			got = append(got, p.items[:n]...)
			return n
		},
	)
	return got, res, err
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
	items := make([]int, 100)
	for i := range items {
		items[i] = i
	}
	calls := 0
	first := makePages(items, 10, &calls)

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
	items := make([]int, 100)
	for i := range items {
		items[i] = i
	}
	calls := 0
	first := makePages(items, 10, &calls)

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
