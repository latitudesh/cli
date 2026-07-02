package events

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/latitudesh/latitudesh-go-sdk/models/components"
	"github.com/latitudesh/latitudesh-go-sdk/models/operations"
	"github.com/latitudesh/lsh/cli"
	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/internal/renderer"
	"github.com/latitudesh/lsh/internal/utils"
	"github.com/latitudesh/lsh/internal/wait"
	cobra "github.com/spf13/cobra"
)

const (
	eventsPageSize = 100
	maxEventsPages = 50
	maxEventsTotal = eventsPageSize * maxEventsPages
)

func NewListCmd() *cobra.Command {
	op := ListEventsOperation{}
	cmd := &cobra.Command{
		Long:  "List all events in the team. Events are returned newest first.\n",
		RunE:  op.run,
		Short: "List events",
		Example: `  lsh events list --since 24h --target-type Server
  lsh events list --author user@example.com
  lsh events list --project my-project --action update --since 2026-06-01
  lsh events list --follow --target-id sv_xxxx`,
		Use:         "list",
		Annotations: map[string]string{cli.ProjectOptionalAnnotation: "true"},
	}

	cmd.Flags().BoolP("follow", "f", false, "stream new events as they occur (polls forward; Ctrl+C to stop)")
	cmd.Flags().String("author", "", "Filter by author ID or email")
	cmd.Flags().String("project", "", "Filter by project ID or slug")
	cmd.Flags().StringSlice("target-type", nil, "Filter by target type (repeatable), e.g. servers, projects, virtual_networks")
	cmd.Flags().String("target-id", "", "Filter by target ID")
	cmd.Flags().String("action", "", "Filter by action, e.g. servers.create")
	cmd.Flags().String("since", "", "Only events created after this point: a duration (24h, 7d) or an ISO date (2026-06-01)")
	cmd.Flags().String("until", "", "Only events created before this point: a duration (24h, 7d) or an ISO date (2026-06-01)")

	return cmd
}

type ListEventsOperation struct{}

func (o *ListEventsOperation) run(cmd *cobra.Command, args []string) error {
	request, err := buildEventsRequest(cmd, time.Now())
	if err != nil {
		utils.PrintError(err)
		return err
	}

	if lsh.DryRun {
		lsh.LogDebugf("dry-run flag specified. Skip sending request.")
		return nil
	}

	if follow, _ := cmd.Flags().GetBool("follow"); follow {
		return followEvents(request, time.Now())
	}

	client := lsh.NewClient()
	ctx := context.Background()

	lshEvents := Events{}
	page := int64(1)
	for {
		request.PageNumber = &page

		response, err := client.Events.List(ctx, *request, operations.WithRetries(lsh.RetryConfig()))
		if err != nil {
			// A failure halfway through pagination would discard every event
			// already fetched; degrade to a partial result with a warning.
			if len(lshEvents.Data) > 0 {
				fmt.Fprintf(os.Stderr, "warning: the events API returned an error on page %d; showing the %d events fetched so far\n", page, len(lshEvents.Data))
				break
			}
			utils.PrintError(err)
			return err
		}

		var pageData []components.EventData
		if response.Events != nil {
			pageData = response.Events.Data
		}

		for i := range pageData {
			lshEvents.Data = append(lshEvents.Data, &Event{EventData: pageData[i]})
		}

		if len(pageData) < eventsPageSize {
			break
		}
		if page >= maxEventsPages {
			fmt.Fprintf(os.Stderr, "warning: stopped after %d events — narrow the range with --since/--until to see older events\n", maxEventsTotal)
			break
		}
		page++
	}

	if !lsh.Debug {
		utils.Render(lshEvents.GetData())
	}

	return nil
}

func buildEventsRequest(cmd *cobra.Command, now time.Time) (*operations.GetEventsRequest, error) {
	pageSize := int64(eventsPageSize)
	request := operations.GetEventsRequest{
		PageSize: &pageSize,
	}

	setString := func(flag string, target **string) {
		if cmd.Flags().Changed(flag) {
			value, _ := cmd.Flags().GetString(flag)
			*target = &value
		}
	}

	setString("author", &request.FilterAuthor)
	setString("project", &request.FilterProject)
	setString("target-id", &request.FilterTargetID)
	setString("action", &request.FilterAction)

	if cmd.Flags().Changed("target-type") {
		request.FilterTargetName, _ = cmd.Flags().GetStringSlice("target-type")
	}

	if cmd.Flags().Changed("since") {
		value, _ := cmd.Flags().GetString("since")
		t, err := utils.ParseTimeRef(value, now)
		if err != nil {
			return nil, fmt.Errorf("invalid --since: %w", err)
		}
		gte := utils.FormatISO8601(t)
		request.FilterCreatedAtGte = &gte
	}

	if cmd.Flags().Changed("until") {
		value, _ := cmd.Flags().GetString("until")
		t, err := utils.ParseTimeRef(value, now)
		if err != nil {
			return nil, fmt.Errorf("invalid --until: %w", err)
		}
		lte := utils.FormatISO8601(t)
		request.FilterCreatedAtLte = &lte
	}

	return &request, nil
}

// followEvents streams new events forever, advancing filter[created_at][gte]
// past the newest event seen and printing each new event in chronological
// order. It stops cleanly on Ctrl+C (SIGINT/SIGTERM).
//
// The events endpoint orders newest-first and has only second-granular
// timestamps, so the cursor is inclusive (gte) and we de-duplicate by event ID
// within the boundary second to avoid reprinting events we already showed.
func followEvents(request *operations.GetEventsRequest, now time.Time) error {
	client := lsh.NewClient()
	ctx, cancel := wait.SignalContext(context.Background())
	defer cancel()

	// Streaming has no upper bound; an --until would only stop the tail early.
	// Make the drop explicit instead of silently ignoring it.
	if request.FilterCreatedAtLte != nil {
		fmt.Fprintln(os.Stderr, "warning: --until is ignored with --follow")
		request.FilterCreatedAtLte = nil
	}
	pageSize := int64(eventsPageSize)
	request.PageSize = &pageSize
	page := int64(1)
	request.PageNumber = &page

	// Without an explicit --since we only want events from now on, so the user
	// isn't flooded with history. With --since, the first poll seeds from there.
	// The probe re-derives filter[created_at][gte] from the cursor every poll.
	deduper := newEventDeduper(followCursor(request, now))

	// --follow streams plain text; structured formats don't fit an open-ended
	// tail, so make the bypass explicit instead of silently ignoring -o.
	if f := renderer.ResolveFormat(); f.IsStructured() {
		fmt.Fprintf(os.Stderr, "warning: --follow streams plain text; -o %s is ignored\n", f)
	}

	fmt.Fprintln(os.Stderr, "Following events… press Ctrl+C to stop")

	// Cap the poll at 5s to honor the freshness target, with a 1s floor (the
	// shared pollFloor) keeping us within the 1 req/s budget.
	backoff := wait.Backoff{Initial: 2 * time.Second, Max: 5 * time.Second, Factor: 1.5}

	// We only fetch the first page each poll. A full page means more matching
	// events may exist beyond it; under high event rates the older ones fall
	// below the advancing cursor and are skipped. Warn once so the gap isn't
	// silent.
	truncationWarned := false

	err := wait.Poll(ctx, backoff, func(ctx context.Context) (bool, error) {
		gte := utils.FormatISO8601(deduper.cursor)
		request.FilterCreatedAtGte = &gte

		response, err := client.Events.List(ctx, *request, operations.WithRetries(lsh.RetryConfig()))
		if err != nil {
			// A transient failure shouldn't kill a long-running stream.
			fmt.Fprintf(os.Stderr, "warning: events poll failed: %v\n", err)
			return false, nil
		}

		var data []components.EventData
		if response.Events != nil {
			data = response.Events.Data
		}

		if len(data) >= eventsPageSize && !truncationWarned {
			fmt.Fprintf(os.Stderr, "warning: more than %d events in a single interval; some older events may be skipped — narrow the filter to follow them all\n", eventsPageSize)
			truncationWarned = true
		}

		for _, ev := range deduper.next(data) {
			printEventLine(ev)
		}
		return false, nil // follow forever
	})

	if errors.Is(err, wait.ErrCanceled) {
		return nil // clean Ctrl+C
	}
	return err
}

// eventDeduper turns successive newest-first event pages into a de-duplicated,
// oldest-first stream. It advances an inclusive, second-granular cursor and —
// for events whose timestamp can't be parsed, and so can't ride the cursor —
// tracks them in a persistent set so each is emitted exactly once.
type eventDeduper struct {
	cursor      time.Time
	printed     map[string]struct{} // keys at the cursor second (rebuilt each page)
	seenUndated map[string]struct{} // keys with no parseable time (persistent)
}

func newEventDeduper(cursor time.Time) *eventDeduper {
	return &eventDeduper{
		cursor:      cursor,
		printed:     map[string]struct{}{},
		seenUndated: map[string]struct{}{},
	}
}

// next returns the events from page (API order, newest-first) not yet emitted,
// in chronological (oldest-first) order, advancing the cursor past the newest.
func (d *eventDeduper) next(page []components.EventData) []*components.EventData {
	newMax := d.cursor
	nextPrinted := map[string]struct{}{}
	var out []*components.EventData

	for i := len(page) - 1; i >= 0; i-- {
		ev := &page[i]
		key := dedupKey(ev)
		ts, ok := eventTime(ev)

		// Undated events can't ride the cursor; emit each exactly once.
		if !ok {
			if _, seen := d.seenUndated[key]; seen {
				continue
			}
			out = append(out, ev)
			d.seenUndated[key] = struct{}{}
			continue
		}

		if _, seen := d.printed[key]; seen {
			// Already shown; keep tracking it while it sits on the boundary second.
			if ts.Equal(newMax) {
				nextPrinted[key] = struct{}{}
			}
			continue
		}

		out = append(out, ev)
		if ts.After(newMax) {
			newMax = ts
			// Boundary second moved forward; drop stale keys.
			nextPrinted = map[string]struct{}{}
		}
		if ts.Equal(newMax) {
			nextPrinted[key] = struct{}{}
		}
	}

	// If the cursor didn't advance (empty page, a page of only undated events,
	// or a transient API inconsistency), keep the boundary-second keys we already
	// knew so those events aren't re-emitted when they reappear on a later poll.
	if newMax.Equal(d.cursor) {
		for k := range d.printed {
			nextPrinted[k] = struct{}{}
		}
	}
	d.cursor = newMax
	d.printed = nextPrinted

	// Bound the persistent undated set. Malformed timestamps should be rare, but
	// over a long-running stream this guards against unbounded growth; clearing
	// may re-emit a handful of undated events once — an acceptable trade.
	const maxUndated = 4096
	if len(d.seenUndated) > maxUndated {
		d.seenUndated = map[string]struct{}{}
	}

	return out
}

// followCursor returns the starting cursor for the --follow stream: the --since
// instant when one was supplied (request.FilterCreatedAtGte), otherwise now.
// The result is second-truncated UTC to match the API's timestamp granularity.
func followCursor(request *operations.GetEventsRequest, now time.Time) time.Time {
	if request.FilterCreatedAtGte != nil {
		if t, ok := parseEventTimestamp(*request.FilterCreatedAtGte); ok {
			return t
		}
	}
	return now.UTC().Truncate(time.Second)
}

// eventTime parses an event's created_at into a second-truncated UTC time.
func eventTime(ev *components.EventData) (time.Time, bool) {
	if ev.Attributes == nil || ev.Attributes.CreatedAt == nil {
		return time.Time{}, false
	}
	return parseEventTimestamp(*ev.Attributes.CreatedAt)
}

// parseEventTimestamp parses an events-API timestamp into a second-truncated
// UTC time, tolerating the formats the endpoint may emit (with or without a
// timezone). Shared by followCursor and eventTime so the two never diverge.
func parseEventTimestamp(s string) (time.Time, bool) {
	layouts := []string{
		time.RFC3339, // == "2006-01-02T15:04:05Z07:00"
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05 -0700 MST",
		"2006-01-02 15:04:05 -0700",
		"2006-01-02 15:04:05",
	}
	for _, l := range layouts {
		if t, err := time.Parse(l, s); err == nil {
			return t.UTC().Truncate(time.Second), true
		}
	}
	return time.Time{}, false
}

// dedupKey returns a stable key for de-duplicating events across polls. It
// prefers the event ID; when the API omits it, it falls back to a synthetic
// key so ID-less events aren't reprinted on every poll within a boundary second.
func dedupKey(ev *components.EventData) string {
	if ev.ID != nil && *ev.ID != "" {
		return *ev.ID
	}
	get := func(p *string) string {
		if p != nil {
			return *p
		}
		return ""
	}
	var action, createdAt, targetID string
	if a := ev.Attributes; a != nil {
		action = get(a.Action)
		createdAt = get(a.CreatedAt)
		if a.Target != nil {
			targetID = get(a.Target.ID)
		}
	}
	return fmt.Sprintf("synthetic:%s|%s|%s", action, createdAt, targetID)
}

// printEventLine writes a compact, plain one-line representation of an event to
// stdout — the streaming counterpart to the table renderer.
func printEventLine(ev *components.EventData) {
	get := func(p *string) string {
		if p != nil {
			return *p
		}
		return ""
	}

	var action, createdAt, author, target string
	if a := ev.Attributes; a != nil {
		action = get(a.Action)
		createdAt = get(a.CreatedAt)
		if a.Author != nil {
			author = get(a.Author.Email)
		}
		if a.Target != nil {
			target = get(a.Target.Name)
			if id := get(a.Target.ID); id != "" {
				if target != "" {
					target = fmt.Sprintf("%s (%s)", target, id)
				} else {
					target = id
				}
			}
		}
	}

	// Normalise to a fixed-width RFC3339 string so the action/target columns
	// don't shift when the API mixes timezone-bearing and bare timestamps.
	if t, ok := parseEventTimestamp(createdAt); ok {
		createdAt = t.Format(time.RFC3339)
	}
	fmt.Printf("%-20s  %-24s  %-30s  %s\n", createdAt, action, target, author)
}
