package traffic

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/latitudesh/latitudesh-go-sdk/models/operations"
	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/internal/utils"
	cobra "github.com/spf13/cobra"
)

const defaultTrafficRange = "7d"

func NewListCmd() *cobra.Command {
	op := ListTrafficOperation{}
	cmd := &cobra.Command{
		Long: "Show traffic consumption per region for a project, mirroring the\n" +
			"dashboard: inbound/outbound totals, plus the quota and how much of it has\n" +
			"been used (units follow the project billing method — TB or Mbps).\n\n" +
			"Use --daily for the day-by-day breakdown behind the dashboard chart.\n\n" +
			"A project filter is required.\n\n" +
			"Measured range: the last 7 days by default (from now). Override it with\n" +
			"--since/--until, which accept a duration (24h, 7d, 2w) or an ISO date\n" +
			"(2026-06-01 or 2026-06-01T15:04:05). The range applies to consumption only;\n" +
			"the Quota column always reflects the current limit, whatever range you query.\n",
		RunE:  op.run,
		Short: "Show traffic consumption",
		Example: `  lsh traffic list --project my-project
  lsh traffic list --project my-project --daily
  lsh traffic list --project my-project --since 30d
  lsh traffic list --project my-project --since 2026-05-01 --until 2026-06-01`,
		Use: "list",
	}

	cmd.Flags().String("project", "", "Project ID or slug to filter by")
	cmd.Flags().String("since", defaultTrafficRange, "Start of the measured range: a duration back from now (24h, 7d, 2w) or an ISO date (2026-06-01). Default: 7d")
	cmd.Flags().String("until", "", "End of the measured range: a duration (24h, 7d) or an ISO date (2026-06-01). Default: now")
	cmd.Flags().Bool("daily", false, "Show the day-by-day breakdown per region instead of the per-region summary")

	return cmd
}

type ListTrafficOperation struct{}

func (o *ListTrafficOperation) run(cmd *cobra.Command, args []string) error {
	project, _ := cmd.Flags().GetString("project")
	daily, _ := cmd.Flags().GetBool("daily")

	if project == "" {
		err := fmt.Errorf("provide --project to show traffic consumption")
		utils.PrintError(err)
		return err
	}

	gte, lte, err := resolveTrafficRange(cmd, time.Now())
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

	// The consumption endpoint also accepts a server filter, but quota is
	// keyed by project only — so traffic list is scoped to a project for now.
	response, err := client.Traffic.Get(ctx, gte, lte, nil, &project, operations.WithRetries(lsh.RetryConfig()))
	if err != nil {
		utils.PrintError(err)
		return err
	}

	if lsh.Debug {
		return nil
	}

	// Make the measured window explicit. Goes to stderr so it never
	// pollutes piped table or --json output.
	fmt.Fprintf(os.Stderr, "Traffic measured from %s to %s (UTC)\n", gte, lte)

	if daily {
		utils.Render(dailyTraffic(response.Traffic))
		return nil
	}

	// Per-region summary mirrors the dashboard, which overlays the current
	// quota on top of consumption.
	var lookup *quotaLookup
	quotaResp, qErr := client.Traffic.GetQuota(ctx, &project, operations.WithRetries(lsh.RetryConfig()))
	if qErr != nil {
		fmt.Fprintf(os.Stderr, "warning: could not load traffic quota; showing consumption only: %v\n", qErr)
	} else {
		lookup = buildQuotaLookup(quotaResp.TrafficQuota, project)
	}

	utils.Render(summarizeTraffic(response.Traffic, lookup))
	return nil
}

func resolveTrafficRange(cmd *cobra.Command, now time.Time) (gte string, lte string, err error) {
	since, _ := cmd.Flags().GetString("since")
	from, err := utils.ParseTimeRef(since, now)
	if err != nil {
		return "", "", fmt.Errorf("invalid --since: %w", err)
	}

	to := now
	if cmd.Flags().Changed("until") {
		until, _ := cmd.Flags().GetString("until")
		to, err = utils.ParseTimeRef(until, now)
		if err != nil {
			return "", "", fmt.Errorf("invalid --until: %w", err)
		}
	}

	return utils.FormatISO8601(from), utils.FormatISO8601(to), nil
}
