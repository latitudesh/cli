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

	"github.com/charmbracelet/bubbles/table"
	"github.com/latitudesh/lsh/internal/tui"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// register in your root.go: rootCmd.AddCommand(newPlansCmd())
func newPlansCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "plans", Short: "Inspect Latitude.sh server plans"}
	cmd.AddCommand(newPlansListCmd())
	cmd.AddCommand(newPlansAvailabilityCmd())
	return cmd
}

func newPlansListCmd() *cobra.Command {
	var (
		gpu        bool
		inStock    bool
		location   string
		name       string
		slug       string
		stockLevel string
		diskEql    int
		diskGte    int
		diskLte    int
		ramEql     int
		ramGte     int
		ramLte     int
		token      string
	)

	c := &cobra.Command{
		Use:   "list",
		Short: "List available plans in grouped format",
		RunE: func(cmd *cobra.Command, args []string) error {
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

			grouped := groupPlans(plans, gpu, name, slug, inStock, location, stockLevel, diskEql, diskGte, diskLte, ramEql, ramGte, ramLte)
			renderGroupedPlans(grouped)
			return nil
		},
	}

	c.Flags().BoolVar(&gpu, "gpu", false, "Filter by the existence of an associated GPU")
	c.Flags().BoolVar(&inStock, "in_stock", false, "The stock available at the site to filter by")
	c.Flags().StringVar(&location, "location", "", "The location of the site to filter by")
	c.Flags().StringVar(&name, "name", "", "The plan name to filter by")
	c.Flags().StringVar(&slug, "slug", "", "The plan slug to filter by")
	c.Flags().StringVar(&stockLevel, "stock_level", "", "Enum: [\"Unavailable\",\"Low\",\"Medium\",\"High\",\"Unique\"]. The stock level at the site to filter by")
	c.Flags().IntVar(&diskEql, "disk_eql", 0, "Filter plans with disk size in Gigabytes equals the provided value.")
	c.Flags().IntVar(&diskGte, "disk_gte", 0, "Filter plans with disk size in Gigabytes greater than or equal the provided value.")
	c.Flags().IntVar(&diskLte, "disk_lte", 0, "Filter plans with disk size in Gigabytes less than or equal the provided value.")
	c.Flags().IntVar(&ramEql, "ram_eql", 0, "Filter plans with RAM size (in GB) equals the provided value.")
	c.Flags().IntVar(&ramGte, "ram_gte", 0, "Filter plans with RAM size (in GB) greater than or equal the provided value.")
	c.Flags().IntVar(&ramLte, "ram_lte", 0, "Filter plans with RAM size (in GB) less than or equal the provided value.")
	c.Flags().StringVar(&token, "token", "", "API token (defaults to LATITUDESH_AUTH_TOKEN)")

	return c
}

func newPlansAvailabilityCmd() *cobra.Command {
	var (
		region     string
		location   string
		inStock    bool
		available  bool
		gpu        bool
		name       string
		slug       string
		stockLevel string
		diskEql    int
		diskGte    int
		diskLte    int
		ramEql     int
		ramGte     int
		ramLte     int
		format     string
		token      string
	)

	c := &cobra.Command{
		Use:   "stock",
		Short: "Show detailed plan availability by location with optional filters",
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

			rows := filterAndFlatten(plans, region, location, inStock, available, gpu, name, slug, stockLevel, diskEql, diskGte, diskLte, ramEql, ramGte, ramLte)

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
				renderStockTable(rows)
				return nil
			}
		},
	}

	c.Flags().StringVar(&region, "region", "", "Filter by region name (e.g. 'Japan')")
	c.Flags().StringVar(&location, "location", "", "The location of the site to filter by")
	c.Flags().StringVar(&location, "site", "", "The location of the site to filter by (alias for --location)")
	c.Flags().BoolVar(&inStock, "in_stock", false, "The stock available at the site to filter by")
	c.Flags().BoolVar(&available, "available", false, "Only include locations marked as available")
	c.Flags().BoolVar(&gpu, "gpu", false, "Filter by the existence of an associated GPU")
	c.Flags().StringVar(&name, "name", "", "The plan name to filter by")
	c.Flags().StringVar(&slug, "slug", "", "The plan slug to filter by")
	c.Flags().StringVar(&stockLevel, "stock_level", "", "Enum: [\"Unavailable\",\"Low\",\"Medium\",\"High\",\"Unique\"]. The stock level at the site to filter by")
	c.Flags().IntVar(&diskEql, "disk_eql", 0, "Filter plans with disk size in Gigabytes equals the provided value.")
	c.Flags().IntVar(&diskGte, "disk_gte", 0, "Filter plans with disk size in Gigabytes greater than or equal the provided value.")
	c.Flags().IntVar(&diskLte, "disk_lte", 0, "Filter plans with disk size in Gigabytes less than or equal the provided value.")
	c.Flags().IntVar(&ramEql, "ram_eql", 0, "Filter plans with RAM size (in GB) equals the provided value.")
	c.Flags().IntVar(&ramGte, "ram_gte", 0, "Filter plans with RAM size (in GB) greater than or equal the provided value.")
	c.Flags().IntVar(&ramLte, "ram_lte", 0, "Filter plans with RAM size (in GB) less than or equal the provided value.")
	c.Flags().StringVar(&format, "format", "table", "Output format: table|csv|json")
	c.Flags().StringVar(&token, "token", "", "API token (defaults to LATITUDESH_AUTH_TOKEN)")

	return c
}

// --- Grouped Plans (for plans list command) ---

type groupedPlan struct {
	Slug        string
	CPU         string
	Drives      string
	NIC         string
	ID          string
	AvailableIn []string
	InStock     []string
	Features    []string
	Memory      string
}

func groupPlans(pr *plansResponse, gpu bool, name, slug string, inStock bool, location, stockLevel string, diskEql, diskGte, diskLte, ramEql, ramGte, ramLte int) []groupedPlan {
	var grouped []groupedPlan

	for _, d := range pr.Data {
		attr := d.Attributes

		// Filter by GPU if requested
		if gpu && !strings.HasPrefix(attr.Slug, "g") {
			continue
		}

		// Filter by name
		if name != "" && !strings.Contains(strings.ToLower(attr.Slug), strings.ToLower(name)) {
			continue
		}

		// Filter by slug (exact match)
		if slug != "" && !strings.EqualFold(attr.Slug, slug) {
			continue
		}

		// Filter by RAM
		if ramEql > 0 && attr.Specs.Memory.Total != ramEql {
			continue
		}
		if ramGte > 0 && attr.Specs.Memory.Total < ramGte {
			continue
		}
		if ramLte > 0 && attr.Specs.Memory.Total > ramLte {
			continue
		}

		// Calculate total disk size for filtering
		totalDiskGB := 0
		for _, dv := range attr.Specs.Drives {
			// Parse disk size (e.g., "3.8TB" -> 3800GB, "960GB" -> 960GB)
			sizeStr := strings.TrimSpace(dv.Size)
			var diskGB int
			if strings.HasSuffix(sizeStr, "TB") {
				var sizeTB float64
				fmt.Sscanf(sizeStr, "%f", &sizeTB)
				diskGB = int(sizeTB * 1000)
			} else if strings.HasSuffix(sizeStr, "GB") {
				fmt.Sscanf(sizeStr, "%d", &diskGB)
			}
			totalDiskGB += dv.Count * diskGB
		}

		// Filter by disk size
		if diskEql > 0 && totalDiskGB != diskEql {
			continue
		}
		if diskGte > 0 && totalDiskGB < diskGte {
			continue
		}
		if diskLte > 0 && totalDiskGB > diskLte {
			continue
		}

		// Collect all available and in-stock locations across all regions
		availableSet := make(map[string]bool)
		inStockSet := make(map[string]bool)
		planHasStockLevel := false

		for _, r := range attr.Regions {
			// Check stock_level filter at region level
			if stockLevel != "" && !strings.EqualFold(r.StockLevel, stockLevel) {
				continue
			}
			if stockLevel != "" && strings.EqualFold(r.StockLevel, stockLevel) {
				planHasStockLevel = true
			}

			for _, loc := range r.Locations.Available {
				// Filter by location if specified
				if location != "" && !strings.EqualFold(loc, location) {
					continue
				}
				availableSet[strings.ToUpper(loc)] = true
			}
			for _, loc := range r.Locations.InStock {
				// Filter by location if specified
				if location != "" && !strings.EqualFold(loc, location) {
					continue
				}
				inStockSet[strings.ToUpper(loc)] = true
			}
		}

		// If stock_level filter is set and no regions matched, skip this plan
		if stockLevel != "" && !planHasStockLevel {
			continue
		}

		// Filter by inStock - if the flag is set and there are no in-stock locations, skip
		if inStock && len(inStockSet) == 0 {
			continue
		}

		// Convert sets to sorted slices
		var available, inStockLocs []string
		for loc := range availableSet {
			available = append(available, loc)
		}
		for loc := range inStockSet {
			inStockLocs = append(inStockLocs, loc)
		}
		sort.Strings(available)
		sort.Strings(inStockLocs)

		// Build CPU string
		cpuStr := fmt.Sprintf("%dx %s", attr.Specs.CPU.Count, attr.Specs.CPU.Type)
		if attr.Specs.CPU.Clock.Value != "" {
			cpuStr += " " + attr.Specs.CPU.Clock.Value
		}
		if attr.Specs.CPU.Cores > 0 {
			cpuStr += fmt.Sprintf(" (%d cores)", attr.Specs.CPU.Cores)
		}

		// Build drives string
		var driveStrings []string
		for _, dv := range attr.Specs.Drives {
			driveStrings = append(driveStrings, fmt.Sprintf("%dx %s %s", dv.Count, dv.Type, dv.Size))
		}
		drivesStr := strings.Join(driveStrings, "\n")

		// Extract features (we'll assume ssh, raid, user_data for now - these come from the API usually)
		features := []string{"ssh", "user_data"}
		if len(attr.Specs.Drives) > 1 {
			features = append(features, "raid")
		}

		// Get first region's NIC info (assuming it's consistent)
		nicStr := ""
		if len(attr.Regions) > 0 {
			// NIC info isn't in the struct, we'll leave it empty or could parse from another field
			nicStr = "N/A"
		}

		grouped = append(grouped, groupedPlan{
			Slug:        attr.Slug,
			CPU:         cpuStr,
			Drives:      drivesStr,
			NIC:         nicStr,
			ID:          d.ID,
			Memory:      fmt.Sprintf("%dGB", attr.Specs.Memory.Total),
			AvailableIn: available,
			InStock:     inStockLocs,
			Features:    features,
		})
	}

	return grouped
}

func renderGroupedPlans(plans []groupedPlan) {
	if len(plans) == 0 {
		fmt.Println("\nNo plans found matching your filters.")
		return
	}

	if os.Getenv("LSH_CLASSIC_OUTPUT") == "true" {
		renderGroupedPlansClassic(plans)
		return
	}

	columns := []table.Column{
		{Title: "SLUG", Width: 20},
		{Title: "CPU", Width: 25},
		{Title: "DRIVES", Width: 30},
		{Title: "NIC", Width: 10},
		{Title: "ID", Width: 18},
		{Title: "FEATURES", Width: 15},
		{Title: "MEMORY", Width: 10},
		{Title: "AVAILABLE IN", Width: 40},
		{Title: "IN STOCK", Width: 30},
	}

	var rows []table.Row
	var originalPlans []map[string]string

	for _, p := range plans {
		availStr := wrapLocationsSmartLimit(p.AvailableIn, 4)
		stockStr := wrapLocationsSmartLimit(p.InStock, 3)

		rows = append(rows, table.Row{
			p.Slug,
			p.CPU,
			p.Drives,
			p.NIC,
			p.ID,
			strings.Join(p.Features, ", "),
			p.Memory,
			availStr,
			stockStr,
		})

		originalPlans = append(originalPlans, map[string]string{
			"SLUG":         p.Slug,
			"CPU":          p.CPU,
			"DRIVES":       p.Drives,
			"NIC":          p.NIC,
			"ID":           p.ID,
			"FEATURES":     strings.Join(p.Features, ", "),
			"MEMORY":       p.Memory,
			"AVAILABLE IN": strings.Join(p.AvailableIn, ", "),
			"IN STOCK":     strings.Join(p.InStock, ", "),
		})
	}

	tui.RunPlansTable("Available Plans", columns, rows, originalPlans)
}

func wrapLocations(locs []string, maxPerLine int) string {
	if len(locs) == 0 {
		return ""
	}

	var lines []string
	var currentLine []string

	for _, loc := range locs {
		currentLine = append(currentLine, loc)
		if len(currentLine) >= maxPerLine {
			lines = append(lines, strings.Join(currentLine, ", "))
			currentLine = nil
		}
	}

	if len(currentLine) > 0 {
		lines = append(lines, strings.Join(currentLine, ", "))
	}

	return strings.Join(lines, ",\n")
}

func wrapLocationsSimple(locs []string, maxLen int) string {
	if len(locs) == 0 {
		return ""
	}

	joined := strings.Join(locs, ", ")
	if len(joined) > maxLen {
		return joined[:maxLen-3] + "..."
	}
	return joined
}

func wrapLocationsSmartLimit(locs []string, maxDisplay int) string {
	if len(locs) == 0 {
		return ""
	}

	if len(locs) <= maxDisplay {
		return strings.Join(locs, ", ")
	}

	displayed := strings.Join(locs[:maxDisplay], ", ")
	remaining := len(locs) - maxDisplay
	return fmt.Sprintf("%s, +%d", displayed, remaining)
}

func renderGroupedPlansClassic(plans []groupedPlan) {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"SLUG", "CPU", "DRIVES", "NIC", "ID", "FEATURES", "MEMORY", "AVAILABLE IN", "IN STOCK"})
	table.SetRowLine(true)
	table.SetColWidth(18)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetAutoFormatHeaders(true)
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)

	for _, p := range plans {
		availStr := wrapLocations(p.AvailableIn, 20)
		stockStr := wrapLocations(p.InStock, 10)

		table.Append([]string{
			p.Slug,
			p.CPU,
			p.Drives,
			p.NIC,
			p.ID,
			strings.Join(p.Features, "\n"),
			p.Memory,
			availStr,
			stockStr,
		})
	}

	fmt.Println()
	table.Render()
	fmt.Printf("\nTotal: %d plans\n\n", len(plans))
}

func renderStockTable(rows []flatRow) {
	if len(rows) == 0 {
		fmt.Println("\nNo plans found matching your filters.")
		return
	}

	if os.Getenv("LSH_CLASSIC_OUTPUT") == "true" {
		renderStockTableClassic(rows)
		return
	}

	// Converter para formato Bubble Tea
	columns := []table.Column{
		{Title: "PLAN", Width: 20},
		{Title: "ID", Width: 18},
		{Title: "CPU", Width: 25},
		{Title: "RAM", Width: 10},
		{Title: "DRIVES", Width: 20},
		{Title: "FEATURES", Width: 15},
		{Title: "STOCK", Width: 12},
		{Title: "PRICE/MO", Width: 12},
		{Title: "REGION", Width: 15},
		{Title: "LOCATION", Width: 20},
	}

	var tableRows []table.Row
	var originalPlans []map[string]string

	for _, r := range rows {
		// Format CPU
		cpuInfo := fmt.Sprintf("%dx %s", r.CPUCount, r.CPUType)
		if r.CPUClock != "" {
			cpuInfo += " " + r.CPUClock
		}

		// Format RAM
		ramInfo := fmt.Sprintf("%dGB", r.MemoryTotalGB)

		// Format drives
		drivesInfo := strings.Join(r.DriveTypes, ", ")

		// Features
		features := []string{"ssh", "user_data"}
		if r.DriveCount > 1 {
			features = append(features, "raid")
		}
		featuresInfo := strings.Join(features, ", ")

		// Stock
		stockDisplay := strings.ToUpper(r.StockLevel)

		// Price
		priceDisplay := fmt.Sprintf("$%s", r.MonthlyUSD)

		tableRows = append(tableRows, table.Row{
			r.PlanSlug,
			r.PlanID,
			cpuInfo,
			ramInfo,
			drivesInfo,
			featuresInfo,
			stockDisplay,
			priceDisplay,
			r.Region,
			r.Location,
		})

		originalPlans = append(originalPlans, map[string]string{
			"PLAN":     r.PlanSlug,
			"ID":       r.PlanID,
			"CPU":      cpuInfo,
			"RAM":      ramInfo,
			"DRIVES":   drivesInfo,
			"FEATURES": featuresInfo,
			"STOCK":    stockDisplay,
			"PRICE/MO": priceDisplay,
			"REGION":   r.Region,
			"LOCATION": r.Location,
		})
	}

	tui.RunPlansTable("Plans Availability", columns, tableRows, originalPlans)
}

func renderStockTableClassic(rows []flatRow) {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"PLAN", "ID", "CPU", "RAM", "DRIVES", "FEATURES", "STOCK", "PRICE/MO", "REGION", "LOCATION"})
	table.SetRowLine(true)
	table.SetAutoWrapText(false)
	table.SetAutoFormatHeaders(true)
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetCenterSeparator("|")
	table.SetColumnSeparator("|")
	table.SetRowSeparator("-")

	for _, r := range rows {
		// Format CPU info more compactly
		cpuInfo := fmt.Sprintf("%dx %s", r.CPUCount, r.CPUType)
		if r.CPUClock != "" {
			cpuInfo += fmt.Sprintf("\n%s", r.CPUClock)
		}
		if r.CPUCores > 0 {
			cpuInfo += fmt.Sprintf("\n(%d cores)", r.CPUCores)
		}

		// Format RAM
		ramInfo := fmt.Sprintf("%dGB", r.MemoryTotalGB)

		// Format drives
		drivesInfo := strings.Join(r.DriveTypes, "\n")

		// Format features
		features := []string{"ssh", "user_data"}
		if r.DriveCount > 1 {
			features = append(features, "raid")
		}
		featuresInfo := strings.Join(features, "\n")

		// Format stock level
		stockDisplay := strings.ToUpper(r.StockLevel)

		// Format price
		priceDisplay := fmt.Sprintf("$%s", r.MonthlyUSD)

		table.Append([]string{
			r.PlanSlug,
			r.PlanID,
			cpuInfo,
			ramInfo,
			drivesInfo,
			featuresInfo,
			stockDisplay,
			priceDisplay,
			r.Region,
			r.Location,
		})
	}

	fmt.Println()
	table.Render()
	fmt.Printf("\nTotal: %d results\n\n", len(rows))
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
		ID         string `json:"id"`
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
	PlanID        string   `json:"plan_id"`
	CPUCount      int      `json:"cpu_count"`
	CPUCores      int      `json:"cpu_cores"`
	CPUClock      string   `json:"cpu_clock"`
	CPUType       string   `json:"cpu_type"`
	MemoryTotalGB int      `json:"memory_total_gb"`
	DriveTypes    []string `json:"drive_types"`
	DriveCount    int      `json:"drive_count"`
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

func filterAndFlatten(pr *plansResponse, regionName, loc string, inStock, available, gpu bool, name, slug, stockLevel string, diskEql, diskGte, diskLte, ramEql, ramGte, ramLte int) []flatRow {
	rows := make([]flatRow, 0)
	for _, d := range pr.Data {
		attr := d.Attributes

		// Filter by GPU if requested - plans with GPU typically start with 'g' (e.g., g3-a100-large, g4-rtx6kpro-large)
		if gpu && !strings.HasPrefix(attr.Slug, "g") {
			continue
		}

		// Filter by name
		if name != "" && !strings.Contains(strings.ToLower(attr.Slug), strings.ToLower(name)) {
			continue
		}

		// Filter by slug (exact match)
		if slug != "" && !strings.EqualFold(attr.Slug, slug) {
			continue
		}

		// Filter by RAM
		if ramEql > 0 && attr.Specs.Memory.Total != ramEql {
			continue
		}
		if ramGte > 0 && attr.Specs.Memory.Total < ramGte {
			continue
		}
		if ramLte > 0 && attr.Specs.Memory.Total > ramLte {
			continue
		}

		// Calculate total disk size for filtering
		totalDiskGB := 0
		for _, dv := range attr.Specs.Drives {
			// Parse disk size (e.g., "3.8TB" -> 3800GB, "960GB" -> 960GB)
			sizeStr := strings.TrimSpace(dv.Size)
			var diskGB int
			if strings.HasSuffix(sizeStr, "TB") {
				var sizeTB float64
				fmt.Sscanf(sizeStr, "%f", &sizeTB)
				diskGB = int(sizeTB * 1000)
			} else if strings.HasSuffix(sizeStr, "GB") {
				fmt.Sscanf(sizeStr, "%d", &diskGB)
			}
			totalDiskGB += dv.Count * diskGB
		}

		// Filter by disk size
		if diskEql > 0 && totalDiskGB != diskEql {
			continue
		}
		if diskGte > 0 && totalDiskGB < diskGte {
			continue
		}
		if diskLte > 0 && totalDiskGB > diskLte {
			continue
		}

		driveStrings := make([]string, 0, len(attr.Specs.Drives))
		for _, dv := range attr.Specs.Drives {
			driveStrings = append(driveStrings, fmt.Sprintf("%dx%s %s", dv.Count, dv.Size, dv.Type))
		}

		for _, r := range attr.Regions {
			if regionName != "" && !strings.EqualFold(r.Name, regionName) {
				continue
			}

			// Filter by stock level
			if stockLevel != "" && !strings.EqualFold(r.StockLevel, stockLevel) {
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
					PlanID:        d.ID,
					CPUCount:      attr.Specs.CPU.Count,
					CPUCores:      attr.Specs.CPU.Cores,
					CPUClock:      attr.Specs.CPU.Clock.Value,
					CPUType:       attr.Specs.CPU.Type,
					MemoryTotalGB: attr.Specs.Memory.Total,
					DriveTypes:    driveStrings,
					DriveCount:    len(attr.Specs.Drives),
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
