package cli

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// register in your root.go: rootCmd.AddCommand(newPlansCmd())
func newPlansCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "plans", Short: "Inspect Latitude.sh server plans"}
	cmd.AddCommand(newPlansListCmd())
	return cmd
}

func newPlansListCmd() *cobra.Command {
	var (
		region    string
		location  string
		inStock   bool
		available bool
		format    string
		token     string
	)

	c := &cobra.Command{
		Use:   "list",
		Short: "List plans with optional filters (region/location/stock) and choose output format",
		RunE: func(cmd *cobra.Command, args []string) error {
			if format == "" {
				format = "table"
			}
			if token == "" {
				token = os.Getenv("LATITUDESH_AUTH_TOKEN")
			}
			if token == "" {
				token = viper.GetString("authorization")
			}
			if token == "" {
				return errors.New("missing token: set --token or LATITUDESH_AUTH_TOKEN or run 'lsh login <token>'")
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 20*time.Second)
			defer cancel()

			plans, err := fetchPlans(ctx, token)
			if err != nil {
				return err
			}

			rows := filterAndFlatten(plans, region, location, inStock, available)

			switch strings.ToLower(format) {
			case "json":
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(rows)
			case "csv":
				w := csv.NewWriter(os.Stdout)
				_ = w.Write([]string{"plan_slug", "cpu_count", "cpu_cores", "cpu_clock", "cpu_type", "memory_total_gb", "drive_types", "stock_level", "monthly_usd", "region", "location"})
				for _, r := range rows {
					_ = w.Write([]string{r.PlanSlug, itoa(r.CPUCount), itoa(r.CPUCores), r.CPUClock, r.CPUType, itoa(r.MemoryTotalGB), strings.Join(r.DriveTypes, "; "), r.StockLevel, r.MonthlyUSD, r.Region, r.Location})
				}
				w.Flush()
				return w.Error()
			default:
				// pretty table (no third-party deps)
				fmt.Fprintf(os.Stdout, "%s\n", "PLAN SLUG                      CPU  CORES  CLOCK    TYPE           RAM(GB)  DRIVES                              STOCK     PRICE(USD)  REGION  LOC")
				fmt.Fprintf(os.Stdout, "%s\n", strings.Repeat("-", 120))
				for _, r := range rows {
					fmt.Fprintf(os.Stdout, "%-30s %3d  %5d  %-7s %-14s %7d  %-35s %-9s %-11s %-6s %-4s\n",
						r.PlanSlug, r.CPUCount, r.CPUCores, r.CPUClock, r.CPUType, r.MemoryTotalGB, strings.Join(r.DriveTypes, "; "), r.StockLevel, r.MonthlyUSD, r.Region, r.Location)
				}
				return nil
			}
		},
	}

	c.Flags().StringVar(&region, "region", "", "Filter by region name (e.g. 'Japan')")
	c.Flags().StringVar(&location, "location", "", "Filter by location slug (e.g. 'TYO3')")
	c.Flags().BoolVar(&inStock, "in-stock", false, "Only include locations currently in stock")
	c.Flags().BoolVar(&available, "available", false, "Only include locations marked as available")
	c.Flags().StringVar(&format, "format", "table", "Output format: table|csv|json")
	c.Flags().StringVar(&token, "token", "", "API token (defaults to LATITUDESH_AUTH_TOKEN)")

	return c
}

// --- API layer ---

// Minimal shapes that match the /plans response (only what we need)

// flexibleString can unmarshal from both string and number
type flexibleString struct {
	Value string
}

func (fs *flexibleString) UnmarshalJSON(data []byte) error {
	// Try to unmarshal as string first
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		fs.Value = s
		return nil
	}
	// Try as number (float64 for JSON numbers)
	var f float64
	if err := json.Unmarshal(data, &f); err == nil {
		fs.Value = fmt.Sprintf("%.2f", f)
		return nil
	}
	// Try as int
	var i int
	if err := json.Unmarshal(data, &i); err == nil {
		fs.Value = fmt.Sprintf("%d", i)
		return nil
	}
	return fmt.Errorf("cannot unmarshal %s into flexibleString", string(data))
}

type plansResponse struct {
	Data []struct {
		Attributes struct {
			Slug  string `json:"slug"`
			Specs struct {
				CPU struct {
					Count int            `json:"count"`
					Cores int            `json:"cores"`
					Clock flexibleString `json:"clock"`
					Type  string         `json:"type"`
				} `json:"cpu"`
				Memory struct {
					Total int `json:"total"`
				} `json:"memory"`
				Drives []struct {
					Count int    `json:"count"`
					Size  string `json:"size"`
					Type  string `json:"type"`
				} `json:"drives"`
			} `json:"specs"`
			Regions []struct {
				Name      string `json:"name"`
				Locations struct {
					Available []string `json:"available"`
					InStock   []string `json:"in_stock"`
				} `json:"locations"`
				Pricing struct {
					USD struct {
						Month flexibleString `json:"month"`
					} `json:"USD"`
				} `json:"pricing"`
				StockLevel string `json:"stock_level"`
			} `json:"regions"`
		} `json:"attributes"`
	} `json:"data"`
}

type flatRow struct {
	PlanSlug      string   `json:"plan_slug"`
	CPUCount      int      `json:"cpu_count"`
	CPUCores      int      `json:"cpu_cores"`
	CPUClock      string   `json:"cpu_clock"`
	CPUType       string   `json:"cpu_type"`
	MemoryTotalGB int      `json:"memory_total_gb"`
	DriveTypes    []string `json:"drive_types"`
	StockLevel    string   `json:"stock_level"`
	MonthlyUSD    string   `json:"monthly_usd"`
	Region        string   `json:"region"`
	Location      string   `json:"location"`
}

func fetchPlans(ctx context.Context, token string) (*plansResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.latitude.sh/plans", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", token)
	req.Header.Set("accept", "application/vnd.api+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("unexpected status %s", resp.Status)
	}
	var out plansResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

func filterAndFlatten(pr *plansResponse, regionName, loc string, inStock, available bool) []flatRow {
	rows := make([]flatRow, 0)
	for _, d := range pr.Data {
		attr := d.Attributes
		driveStrings := make([]string, 0, len(attr.Specs.Drives))
		for _, dv := range attr.Specs.Drives {
			driveStrings = append(driveStrings, fmt.Sprintf("%dx%s %s", dv.Count, dv.Size, dv.Type))
		}

		for _, r := range attr.Regions {
			if regionName != "" && !strings.EqualFold(r.Name, regionName) {
				continue
			}

			// build a set for faster lookup
			availSet := toSet(r.Locations.Available)
			stockSet := toSet(r.Locations.InStock)

			// candidate locations are the union of available and in_stock (to avoid missing price/stock info)
			locs := unique(append(append([]string{}, r.Locations.Available...), r.Locations.InStock...))
			for _, l := range locs {
				if loc != "" && !strings.EqualFold(l, loc) {
					continue
				}
				if available && !availSet[strings.ToUpper(l)] {
					continue
				}
				if inStock && !stockSet[strings.ToUpper(l)] {
					continue
				}

				rows = append(rows, flatRow{
					PlanSlug:      attr.Slug,
					CPUCount:      attr.Specs.CPU.Count,
					CPUCores:      attr.Specs.CPU.Cores,
					CPUClock:      attr.Specs.CPU.Clock.Value,
					CPUType:       attr.Specs.CPU.Type,
					MemoryTotalGB: attr.Specs.Memory.Total,
					DriveTypes:    driveStrings,
					StockLevel:    r.StockLevel,
					MonthlyUSD:    r.Pricing.USD.Month.Value,
					Region:        r.Name,
					Location:      strings.ToUpper(l),
				})
			}
		}
	}
	// stable output
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].PlanSlug == rows[j].PlanSlug {
			if rows[i].Region == rows[j].Region {
				return rows[i].Location < rows[j].Location
			}
			return rows[i].Region < rows[j].Region
		}
		return rows[i].PlanSlug < rows[j].PlanSlug
	})
	return rows
}

func itoa(i int) string { return fmt.Sprintf("%d", i) }

func toSet(ss []string) map[string]bool {
	m := make(map[string]bool, len(ss))
	for _, s := range ss {
		m[strings.ToUpper(s)] = true
	}
	return m
}

func unique(ss []string) []string {
	m := map[string]struct{}{}
	out := make([]string, 0, len(ss))
	for _, s := range ss {
		u := strings.ToUpper(s)
		if _, ok := m[u]; ok {
			continue
		}
		m[u] = struct{}{}
		out = append(out, s)
	}
	return out
}
