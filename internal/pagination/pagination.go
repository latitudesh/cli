// Package pagination centralizes the list-pagination controls shared by every
// `list` command: the --page-size, --max-items and --no-paginate flags. Keeping
// the stop-condition logic in one place (mirroring how internal/renderer owns
// output formatting) means each command only describes how to read one page,
// and behaves consistently for scripts, CI and a future MCP server.
package pagination

import (
	"fmt"
	"os"

	"github.com/spf13/viper"
)

// DefaultPageSize is the page[size] requested when --page-size is not set. The
// API default is small (20), which turns large listings into long chains of
// requests; asking for bigger pages keeps the page count (and wall-clock time)
// down while preserving the historical "list everything" behavior.
const DefaultPageSize int64 = 100

// Options holds the resolved pagination controls for a single command run.
type Options struct {
	// PageSize is the number of items requested per API page.
	PageSize int64
	// MaxItems caps the total number of items returned across all pages.
	// Zero means no limit (fetch every page).
	MaxItems int64
	// NoPaginate stops after the first page and reports the next page so the
	// caller can resume manually.
	NoPaginate bool
}

// Validate rejects invalid pagination flag values up front (with a non-zero
// exit) instead of silently clamping them, so automation callers get clear
// feedback. The --page-size flag defaults to a positive value, so a
// non-positive reading means the user set it explicitly.
func Validate() error {
	if viper.GetInt64("page-size") <= 0 {
		return fmt.Errorf("--page-size must be a positive integer")
	}
	if viper.GetInt64("max-items") < 0 {
		return fmt.Errorf("--max-items must be zero or a positive integer")
	}
	return nil
}

// Resolve reads the pagination flags from viper, applying sensible defaults.
func Resolve() Options {
	pageSize := viper.GetInt64("page-size")
	if pageSize <= 0 {
		pageSize = DefaultPageSize
	}
	maxItems := viper.GetInt64("max-items")
	if maxItems < 0 {
		maxItems = 0
	}
	return Options{
		PageSize:   pageSize,
		MaxItems:   maxItems,
		NoPaginate: viper.GetBool("no-paginate"),
	}
}

// Result reports whether more pages exist beyond what was collected and the
// number of the next page (page[number]) the caller could request to continue.
type Result struct {
	HasMore  bool
	NextPage int64
}

// Walk drives pagination over an SDK list response.
//
//   - first is the already-fetched first page (may be nil).
//   - next returns the response's Next closure, or nil when there are no more
//     pages.
//   - onPage appends up to limit rows from a page (limit < 0 means no limit)
//     and returns how many it appended (added) and how many rows the API
//     returned for that page (pageLen), before any client-side filtering or
//     trimming.
//
// Walk honors NoPaginate (stop after the first page) and MaxItems (stop once
// the running total reaches the cap, trimming the final page). It assumes the
// first page corresponds to page[number]=1, which is how every list command
// issues its initial request.
//
// The generated SDK sets Next on every list response and only finds out that
// there is no next page when the closure runs: it returns (nil, nil) for an
// empty page or one shorter than page[size], and issues the request otherwise.
// When Walk stops early it must not call Next (that would fetch a page the
// caller asked it not to), so it applies the same page-length rule itself to
// decide whether results were left behind.
func Walk[R any](
	first *R,
	opts Options,
	next func(*R) func() (*R, error),
	onPage func(page *R, limit int) (added, pageLen int),
) (Result, error) {
	if first == nil {
		return Result{}, nil
	}

	page := first
	total := 0
	pageNum := int64(1)

	for {
		limit := -1
		if opts.MaxItems > 0 {
			limit = int(opts.MaxItems) - total
		}

		added, pageLen := onPage(page, limit)
		total += added
		nextFn := next(page)

		switch {
		case opts.NoPaginate || (opts.MaxItems > 0 && total >= int(opts.MaxItems)):
			// Stopping without fetching further pages (--no-paginate, or the
			// item cap was reached): report where to resume and whether
			// anything was left behind.
			more := nextFn != nil && leftBehind(opts, added, pageLen, limit)
			return Result{HasMore: more, NextPage: pageNum + 1}, nil
		case nextFn == nil:
			return Result{HasMore: false}, nil
		}

		np, err := nextFn()
		if err != nil {
			return Result{}, err
		}
		if np == nil {
			// Some SDK Next closures signal "no more pages" by returning a nil
			// response rather than the closure itself being nil.
			return Result{HasMore: false}, nil
		}
		page = np
		pageNum++
	}
}

// leftBehind reports whether a page Walk stopped on still hides results: rows
// the item cap trimmed off it, or later pages. A page shorter than page[size]
// is the last one (the SDK's Next would return nil for it), so a short page
// that was not trimmed means nothing was left behind. Client-side filters make
// added smaller than pageLen without trimming anything, which is why the API
// page length, not the number of rows kept, decides whether a page is short.
func leftBehind(opts Options, added, pageLen, limit int) bool {
	if pageLen == 0 {
		return false
	}
	trimmed := limit >= 0 && added >= limit && added < pageLen
	short := opts.PageSize > 0 && int64(pageLen) < opts.PageSize
	return trimmed || !short
}

// PrintNextCursor writes a one-line hint to stderr telling the user where to
// resume when --no-paginate stopped before exhausting the results. It goes to
// stderr so structured output on stdout stays a clean, pipeable payload.
func PrintNextCursor(nextPage int64) {
	fmt.Fprintf(os.Stderr,
		"More results available. Next page: %d (use --page-size/--max-items, or drop --no-paginate to fetch all)\n",
		nextPage)
}
