package events

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/latitudesh/latitudesh-go-sdk/models/components"
	"github.com/latitudesh/latitudesh-go-sdk/models/operations"
	"github.com/latitudesh/lsh/cli"
	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/internal/utils"
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
  lsh events list --project my-project --action update --since 2026-06-01`,
		Use:         "list",
		Annotations: map[string]string{cli.ProjectOptionalAnnotation: "true"},
	}

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
