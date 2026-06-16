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
//     and returns how many it appended.
//
// Walk honors NoPaginate (stop after the first page) and MaxItems (stop once
// the running total reaches the cap, trimming the final page). It assumes the
// first page corresponds to page[number]=1, which is how every list command
// issues its initial request.
func Walk[R any](
	first *R,
	opts Options,
	next func(*R) func() (*R, error),
	onPage func(page *R, limit int) (added int),
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

		total += onPage(page, limit)
		nextFn := next(page)

		switch {
		case opts.NoPaginate:
			// Only the first page was requested; report where to resume.
			return Result{HasMore: nextFn != nil, NextPage: pageNum + 1}, nil
		case opts.MaxItems > 0 && total >= int(opts.MaxItems):
			// Item cap reached; do not issue further requests.
			return Result{HasMore: nextFn != nil, NextPage: pageNum + 1}, nil
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

// PrintNextCursor writes a one-line hint to stderr telling the user where to
// resume when --no-paginate stopped before exhausting the results. It goes to
// stderr so structured output on stdout stays a clean, pipeable payload.
func PrintNextCursor(nextPage int64) {
	fmt.Fprintf(os.Stderr,
		"More results available. Next page: %d (use --page-size/--max-items, or drop --no-paginate to fetch all)\n",
		nextPage)
}
