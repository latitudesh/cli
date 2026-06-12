package traffic

import (
	"math"
	"sort"
	"strconv"

	"github.com/latitudesh/latitudesh-go-sdk/models/components"
	"github.com/latitudesh/lsh/internal/output/table"
	"github.com/latitudesh/lsh/internal/renderer"
)

// billingMethod95th is the value the quota endpoint returns for projects
// billed on the 95th-percentile speed model. Anything else (notably
// "Normal") is billed on transferred volume. Mirrors the dashboard's
// BILLING_METHODS constant.
const billingMethod95th = "95th percentile"

// gbPerTB matches the dashboard's convertGBtoTB helper (x / 1000).
const gbPerTB = 1000.0

// TrafficRegionSummary is the per-region headline the dashboard shows on
// each bandwidth card: inbound/outbound totals plus the quota and how
// much of it has been used. Units follow the project billing method —
// TB for volume billing, Mbps for 95th-percentile billing.
type TrafficRegionSummary struct {
	Region           string  `json:"region"`
	BillingMethod    string  `json:"billing_method,omitempty"`
	InboundTb        float64 `json:"inbound_tb"`
	OutboundTb       float64 `json:"outbound_tb"`
	Inbound95thMbps  float64 `json:"inbound_95th_percentile_mbps"`
	Outbound95thMbps float64 `json:"outbound_95th_percentile_mbps"`
	QuotaTotal       *int64  `json:"quota_total,omitempty"`
	UsedPercent      *int64  `json:"used_percent,omitempty"`
}

func (m *TrafficRegionSummary) is95th() bool {
	return m.BillingMethod == billingMethod95th
}

func (m *TrafficRegionSummary) TableRow() table.Row {
	unit := "TB"
	inbound, outbound := m.InboundTb, m.OutboundTb
	if m.is95th() {
		unit = "Mbps"
		inbound, outbound = m.Inbound95thMbps, m.Outbound95thMbps
	}

	billing := m.BillingMethod
	if billing == "" {
		billing = "-"
	}

	quota := "-"
	if m.QuotaTotal != nil {
		quota = formatFloat(float64(*m.QuotaTotal)) + " " + unit
	}

	used := "-"
	if m.UsedPercent != nil {
		used = strconv.FormatInt(*m.UsedPercent, 10) + "%"
	}

	return table.Row{
		"region": table.Cell{
			Label: "Region",
			Value: table.String(m.Region),
		},
		"billing": table.Cell{
			Label: "Billing",
			Value: table.String(billing),
		},
		"inbound": table.Cell{
			Label: "Inbound",
			Value: table.String(formatFloat(inbound) + " " + unit),
		},
		"outbound": table.Cell{
			Label: "Outbound",
			Value: table.String(formatFloat(outbound) + " " + unit),
		},
		"quota": table.Cell{
			Label: "Quota",
			Value: table.String(quota),
		},
		"used": table.Cell{
			Label: "Used",
			Value: table.String(used),
		},
	}
}

// regionQuota holds the per-region quota totals resolved from the quota
// endpoint, alongside the project billing method that decides which unit
// to read.
type regionQuota struct {
	tbTotal   *int64
	mbpsTotal *int64
}

type quotaLookup struct {
	billingMethod string
	byRegion      map[string]regionQuota
}

// buildQuotaLookup resolves the quota entry for the requested project from
// a quota response. With a single project in the payload it uses that one;
// otherwise it matches on the project slug. Returns nil when nothing maps,
// so callers render consumption without quota columns.
func buildQuotaLookup(q *components.TrafficQuota, project string) *quotaLookup {
	if q == nil || q.Data == nil || q.Data.Attributes == nil {
		return nil
	}

	projects := q.Data.Attributes.QuotaPerProject
	if len(projects) == 0 {
		return nil
	}

	var match *components.QuotaPerProject
	if len(projects) == 1 {
		match = &projects[0]
	} else {
		for i := range projects {
			if stringValue(projects[i].ProjectSlug) == project {
				match = &projects[i]
				break
			}
		}
	}
	if match == nil {
		return nil
	}

	lookup := &quotaLookup{
		billingMethod: stringValue(match.BillingMethod),
		byRegion:      map[string]regionQuota{},
	}
	for _, region := range match.QuotaPerRegion {
		rq := regionQuota{}
		if region.QuotaInTb != nil {
			rq.tbTotal = region.QuotaInTb.Total
		}
		if region.QuotaInMbps != nil {
			rq.mbpsTotal = region.QuotaInMbps.Total
		}
		lookup.byRegion[stringValue(region.RegionSlug)] = rq
	}
	return lookup
}

// summarizeTraffic produces one row per region — the dashboard card
// headline — merging consumption totals with the resolved quota. Rows are
// ordered by region slug for stable output.
func summarizeTraffic(t *components.Traffic, lookup *quotaLookup) []renderer.ResponseData {
	var rows []renderer.ResponseData

	if t == nil || t.Data == nil || t.Data.Attributes == nil {
		return rows
	}

	billing := ""
	if lookup != nil {
		billing = lookup.billingMethod
	}

	for _, region := range t.Data.Attributes.Regions {
		regionSlug := stringValue(region.RegionSlug)
		if regionSlug == "" {
			regionSlug = "-"
		}

		summary := &TrafficRegionSummary{
			Region:           regionSlug,
			BillingMethod:    billing,
			InboundTb:        gbToTB(int64Value(region.TotalInboundGb)),
			OutboundTb:       gbToTB(int64Value(region.TotalOutboundGb)),
			Inbound95thMbps:  float64Value(region.TotalInbound95thPercentileMbps),
			Outbound95thMbps: float64Value(region.TotalOutbound95thPercentileMbps),
		}

		applyQuota(summary, lookup, stringValue(region.RegionSlug))
		rows = append(rows, summary)
	}

	sort.Slice(rows, func(i, j int) bool {
		return rows[i].(*TrafficRegionSummary).Region < rows[j].(*TrafficRegionSummary).Region
	})

	return rows
}

// applyQuota attaches the quota total and used-percent to a summary,
// following the dashboard: usage is the outbound figure, percent is
// round(usage / quota * 100), and the unit tracks the billing method.
func applyQuota(summary *TrafficRegionSummary, lookup *quotaLookup, regionSlug string) {
	if lookup == nil {
		return
	}
	rq, ok := lookup.byRegion[regionSlug]
	if !ok {
		return
	}

	var quota *int64
	var usage float64
	if summary.is95th() {
		quota = rq.mbpsTotal
		usage = summary.Outbound95thMbps
	} else {
		quota = rq.tbTotal
		usage = summary.OutboundTb
	}
	if quota == nil {
		return
	}

	summary.QuotaTotal = quota
	if *quota > 0 {
		percent := int64(math.Round(usage / float64(*quota) * 100))
		summary.UsedPercent = &percent
	}
}

// TrafficDay is one day of traffic in one region, the daily breakdown
// behind the dashboard chart. Surfaced by `traffic list --daily`.
type TrafficDay struct {
	Region          string  `json:"region"`
	Date            string  `json:"date"`
	InboundGb       int64   `json:"inbound_gb"`
	OutboundGb      int64   `json:"outbound_gb"`
	AvgInboundMbps  float64 `json:"avg_inbound_speed_mbps"`
	AvgOutboundMbps float64 `json:"avg_outbound_speed_mbps"`
}

func (m *TrafficDay) TableRow() table.Row {
	return table.Row{
		"date": table.Cell{
			Label: "Date",
			Value: table.String(m.Date),
		},
		"region": table.Cell{
			Label: "Region",
			Value: table.String(m.Region),
		},
		"inbound_gb": table.Cell{
			Label: "Inbound (GB)",
			Value: table.Int(m.InboundGb),
		},
		"outbound_gb": table.Cell{
			Label: "Outbound (GB)",
			Value: table.Int(m.OutboundGb),
		},
		"avg_inbound_speed_mbps": table.Cell{
			Label: "Avg In (Mbps)",
			Value: table.Float(m.AvgInboundMbps),
		},
		"avg_outbound_speed_mbps": table.Cell{
			Label: "Avg Out (Mbps)",
			Value: table.Float(m.AvgOutboundMbps),
		},
	}
}

// dailyTraffic flattens the consumption payload to one row per region per
// day, ordered by region then date ascending — the chronological reading
// the dashboard chart shows.
func dailyTraffic(t *components.Traffic) []renderer.ResponseData {
	var rows []renderer.ResponseData

	if t == nil || t.Data == nil || t.Data.Attributes == nil {
		return rows
	}

	for _, region := range t.Data.Attributes.Regions {
		regionSlug := stringValue(region.RegionSlug)
		if regionSlug == "" {
			regionSlug = "-"
		}

		for _, day := range region.Data {
			rows = append(rows, &TrafficDay{
				Region:          regionSlug,
				Date:            stringValue(day.Date),
				InboundGb:       int64Value(day.InboundGb),
				OutboundGb:      int64Value(day.OutboundGb),
				AvgInboundMbps:  float64Value(day.AvgInboundSpeedMbps),
				AvgOutboundMbps: float64Value(day.AvgOutboundSpeedMbps),
			})
		}
	}

	sort.SliceStable(rows, func(i, j int) bool {
		a, b := rows[i].(*TrafficDay), rows[j].(*TrafficDay)
		if a.Region != b.Region {
			return a.Region < b.Region
		}
		return a.Date < b.Date
	})

	return rows
}

// QuotaRow is one project/region quota entry, flattened from the
// nested quota payload. Projects without per-region quotas produce a
// single row with an empty region.
type QuotaRow struct {
	Project           string `json:"project"`
	BillingMethod     string `json:"billing_method"`
	Region            string `json:"region"`
	QuotaTbGranted    int64  `json:"quota_in_tb_granted"`
	QuotaTbAdditional int64  `json:"quota_in_tb_additional"`
	QuotaTbTotal      int64  `json:"quota_in_tb_total"`
	QuotaMbpsTotal    int64  `json:"quota_in_mbps_total"`
}

func (m *QuotaRow) TableRow() table.Row {
	return table.Row{
		"project": table.Cell{
			Label: "Project",
			Value: table.String(m.Project),
		},
		"billing_method": table.Cell{
			Label: "Billing",
			Value: table.String(m.BillingMethod),
		},
		"region": table.Cell{
			Label: "Region",
			Value: table.String(m.Region),
		},
		"quota_in_tb_granted": table.Cell{
			Label: "Granted (TB)",
			Value: table.Int(m.QuotaTbGranted),
		},
		"quota_in_tb_additional": table.Cell{
			Label: "Additional (TB)",
			Value: table.Int(m.QuotaTbAdditional),
		},
		"quota_in_tb_total": table.Cell{
			Label: "Quota (TB)",
			Value: table.Int(m.QuotaTbTotal),
		},
		"quota_in_mbps_total": table.Cell{
			Label: "Quota (Mbps)",
			Value: table.Int(m.QuotaMbpsTotal),
		},
	}
}

func flattenQuota(q *components.TrafficQuota) []renderer.ResponseData {
	var rows []renderer.ResponseData

	if q == nil || q.Data == nil || q.Data.Attributes == nil {
		return rows
	}

	for _, project := range q.Data.Attributes.QuotaPerProject {
		base := QuotaRow{
			Project:       stringValue(project.ProjectSlug),
			BillingMethod: stringValue(project.BillingMethod),
			Region:        "-",
		}

		if len(project.QuotaPerRegion) == 0 {
			row := base
			rows = append(rows, &row)
			continue
		}

		for _, region := range project.QuotaPerRegion {
			row := base
			row.Region = stringValue(region.RegionSlug)
			if quota := region.QuotaInTb; quota != nil {
				row.QuotaTbGranted = int64Value(quota.Granted)
				row.QuotaTbAdditional = int64Value(quota.Additional)
				row.QuotaTbTotal = int64Value(quota.Total)
			}
			if quota := region.QuotaInMbps; quota != nil {
				row.QuotaMbpsTotal = int64Value(quota.Total)
			}
			rows = append(rows, &row)
		}
	}

	return rows
}

// gbToTB converts gigabytes to terabytes rounded to two decimals, matching
// the dashboard's convertGBtoTB(x, 2).
func gbToTB(gb int64) float64 {
	return math.Round(float64(gb)/gbPerTB*100) / 100
}

// formatFloat renders a float without trailing zeros (e.g. 12.5, 133.73, 0),
// matching how the dashboard prints rounded values.
func formatFloat(v float64) string {
	return strconv.FormatFloat(math.Round(v*100)/100, 'f', -1, 64)
}

func stringValue(s *string) string {
	if s != nil {
		return *s
	}
	return ""
}

func int64Value(v *int64) int64 {
	if v != nil {
		return *v
	}
	return 0
}

func float64Value(v *float64) float64 {
	if v != nil {
		return *v
	}
	return 0
}
