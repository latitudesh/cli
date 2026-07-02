package traffic

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/latitudesh/latitudesh-go-sdk/models/components"
)

const normalQuotaFixture = `{
  "data": {
    "id": "quota",
    "type": "traffic_quota",
    "attributes": {
      "quota_per_project": [
        {
          "project_id": "proj_x",
          "project_slug": "my-project",
          "billing_method": "Normal",
          "quota_per_region": [
            {
              "region_slug": "SAO",
              "quota_in_tb": {"granted": 80, "additional": 20, "total": 100},
              "quota_in_mbps": {"granted": 0, "additional": 0, "total": 0}
            }
          ]
        }
      ]
    }
  }
}`

const percentileQuotaFixture = `{
  "data": {
    "id": "quota",
    "type": "traffic_quota",
    "attributes": {
      "quota_per_project": [
        {
          "project_id": "proj_x",
          "project_slug": "my-project",
          "billing_method": "95th percentile",
          "quota_per_region": [
            {
              "region_slug": "SAO",
              "quota_in_tb": {"granted": 0, "additional": 0, "total": 0},
              "quota_in_mbps": {"granted": 150, "additional": 50, "total": 200}
            }
          ]
        }
      ]
    }
  }
}`

const trafficFixture = `{
  "data": {
    "id": "",
    "type": "traffic",
    "attributes": {
      "from_date": 1780444800,
      "to_date": 1781049600,
      "total_inbound_gb": 12,
      "total_outbound_gb": 34,
      "regions": [
        {
          "region_slug": "SAO",
          "total_inbound_gb": 12000,
          "total_outbound_gb": 34000,
          "total_inbound_95th_percentile_mbps": 133.73,
          "total_outbound_95th_percentile_mbps": 144.04,
          "data": [
            {
              "date": "2026-06-03T00:00:00+00:00",
              "inbound_gb": 5,
              "outbound_gb": 14,
              "avg_inbound_speed_mbps": 1.5,
              "avg_outbound_speed_mbps": 4.2
            },
            {
              "date": "2026-06-04T00:00:00+00:00",
              "inbound_gb": 7,
              "outbound_gb": 20,
              "avg_inbound_speed_mbps": 2.1,
              "avg_outbound_speed_mbps": 5.8
            }
          ]
        },
        {
          "region_slug": null,
          "data": [
            {
              "date": "2026-06-03T00:00:00+00:00",
              "inbound_gb": 0,
              "outbound_gb": 0,
              "avg_inbound_speed_mbps": 0.0,
              "avg_outbound_speed_mbps": 0.0
            }
          ]
        }
      ]
    }
  }
}`

func TestDailyTraffic(t *testing.T) {
	var payload components.Traffic
	if err := json.Unmarshal([]byte(trafficFixture), &payload); err != nil {
		t.Fatalf("could not unmarshal fixture: %v", err)
	}

	rows := dailyTraffic(&payload)
	if len(rows) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(rows))
	}

	// Ordered region then date ascending: the null-slug region ("-") sorts
	// before "SAO", and within SAO the earlier day comes first.
	first, ok := rows[0].(*TrafficDay)
	if !ok {
		t.Fatalf("expected *TrafficDay, got %T", rows[0])
	}
	if first.Region != "-" {
		t.Errorf("expected null region (\"-\") to sort first, got %q", first.Region)
	}

	sao := rows[1].(*TrafficDay)
	if sao.Region != "SAO" || sao.Date != "2026-06-03T00:00:00+00:00" ||
		sao.InboundGb != 5 || sao.OutboundGb != 14 ||
		sao.AvgInboundMbps != 1.5 || sao.AvgOutboundMbps != 4.2 {
		t.Errorf("unexpected SAO first day row: %+v", sao)
	}
	if next := rows[2].(*TrafficDay); next.Date < sao.Date {
		t.Errorf("days within a region must ascend, got %q before %q", sao.Date, next.Date)
	}
}

func TestDailyTrafficEmpty(t *testing.T) {
	if rows := dailyTraffic(nil); len(rows) != 0 {
		t.Errorf("expected no rows for nil traffic, got %d", len(rows))
	}
	if rows := dailyTraffic(&components.Traffic{}); len(rows) != 0 {
		t.Errorf("expected no rows for empty traffic, got %d", len(rows))
	}
}

func TestSummarizeTrafficNormalBilling(t *testing.T) {
	var payload components.Traffic
	if err := json.Unmarshal([]byte(trafficFixture), &payload); err != nil {
		t.Fatalf("could not unmarshal fixture: %v", err)
	}
	var quota components.TrafficQuota
	if err := json.Unmarshal([]byte(normalQuotaFixture), &quota); err != nil {
		t.Fatalf("could not unmarshal quota fixture: %v", err)
	}

	lookup := buildQuotaLookup(&quota, "my-project")
	rows := summarizeTraffic(&payload, lookup)
	if len(rows) != 2 {
		t.Fatalf("expected 2 region summaries, got %d", len(rows))
	}

	// Rows sorted by region: "-" then "SAO".
	sao, ok := rows[1].(*TrafficRegionSummary)
	if !ok {
		t.Fatalf("expected *TrafficRegionSummary, got %T", rows[1])
	}
	if sao.Region != "SAO" {
		t.Fatalf("expected SAO row, got %q", sao.Region)
	}
	if sao.InboundTb != 12 || sao.OutboundTb != 34 {
		t.Errorf("expected 12/34 TB, got %v/%v", sao.InboundTb, sao.OutboundTb)
	}
	if sao.QuotaTotal == nil || *sao.QuotaTotal != 100 {
		t.Errorf("expected quota 100 TB, got %v", sao.QuotaTotal)
	}
	// usage = outbound 34 TB of 100 → 34%.
	if sao.UsedPercent == nil || *sao.UsedPercent != 34 {
		t.Errorf("expected used 34%%, got %v", sao.UsedPercent)
	}

	row := sao.TableRow()
	if got := row["inbound"].Value; got != "12 TB" {
		t.Errorf("inbound cell = %q, want \"12 TB\"", got)
	}
	if got := row["used"].Value; got != "34%" {
		t.Errorf("used cell = %q, want \"34%%\"", got)
	}
}

func TestSummarizeTraffic95thBilling(t *testing.T) {
	var payload components.Traffic
	if err := json.Unmarshal([]byte(trafficFixture), &payload); err != nil {
		t.Fatalf("could not unmarshal fixture: %v", err)
	}
	var quota components.TrafficQuota
	if err := json.Unmarshal([]byte(percentileQuotaFixture), &quota); err != nil {
		t.Fatalf("could not unmarshal quota fixture: %v", err)
	}

	lookup := buildQuotaLookup(&quota, "my-project")
	rows := summarizeTraffic(&payload, lookup)
	sao := rows[1].(*TrafficRegionSummary)

	if !sao.is95th() {
		t.Fatalf("expected 95th billing method, got %q", sao.BillingMethod)
	}
	row := sao.TableRow()
	if got := row["outbound"].Value; got != "144.04 Mbps" {
		t.Errorf("outbound cell = %q, want \"144.04 Mbps\"", got)
	}
	// usage = outbound 144.04 Mbps of quota 200 → round(72.02) = 72%.
	if sao.UsedPercent == nil || *sao.UsedPercent != 72 {
		t.Errorf("expected used 72%%, got %v", sao.UsedPercent)
	}
}

func TestSummarizeTrafficWithoutQuota(t *testing.T) {
	var payload components.Traffic
	if err := json.Unmarshal([]byte(trafficFixture), &payload); err != nil {
		t.Fatalf("could not unmarshal fixture: %v", err)
	}

	rows := summarizeTraffic(&payload, nil)
	sao := rows[1].(*TrafficRegionSummary)

	if sao.QuotaTotal != nil || sao.UsedPercent != nil {
		t.Errorf("expected no quota without a lookup, got %+v", sao)
	}
	row := sao.TableRow()
	if row["quota"].Value != "-" || row["used"].Value != "-" {
		t.Errorf("expected placeholder quota/used cells, got %q/%q", row["quota"].Value, row["used"].Value)
	}
}

const quotaFixture = `{
  "data": {
    "id": "quota",
    "type": "traffic_quota",
    "attributes": {
      "quota_per_project": [
        {
          "project_id": "proj_a",
          "project_slug": "with-regions",
          "billing_method": "95th",
          "quota_per_region": [
            {
              "region_id": "loc_x",
              "region_slug": "SAO",
              "quota_in_tb": {"granted": 20, "additional": 5, "total": 25},
              "quota_in_mbps": {"granted": 60, "additional": 16, "total": 76}
            }
          ]
        },
        {
          "project_id": "proj_b",
          "project_slug": "no-regions",
          "billing_method": "Normal",
          "quota_per_region": []
        }
      ]
    }
  }
}`

func TestFlattenQuota(t *testing.T) {
	var payload components.TrafficQuota
	if err := json.Unmarshal([]byte(quotaFixture), &payload); err != nil {
		t.Fatalf("could not unmarshal fixture: %v", err)
	}

	rows := flattenQuota(&payload)
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}

	first, ok := rows[0].(*QuotaRow)
	if !ok {
		t.Fatalf("expected *QuotaRow, got %T", rows[0])
	}
	if first.Project != "with-regions" || first.BillingMethod != "95th" || first.Region != "SAO" ||
		first.QuotaTbGranted != 20 || first.QuotaTbAdditional != 5 || first.QuotaTbTotal != 25 ||
		first.QuotaMbpsTotal != 76 {
		t.Errorf("unexpected first row: %+v", first)
	}

	second := rows[1].(*QuotaRow)
	if second.Project != "no-regions" || second.Region != "-" || second.QuotaTbTotal != 0 {
		t.Errorf("project without regions should produce a single placeholder row, got %+v", second)
	}
}

func TestResolveTrafficRange(t *testing.T) {
	now := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)

	cmd := NewListCmd()
	if err := cmd.Flags().Parse(nil); err != nil {
		t.Fatalf("flag parse error: %v", err)
	}

	gte, lte, err := resolveTrafficRange(cmd, now)
	if err != nil {
		t.Fatalf("resolveTrafficRange returned error: %v", err)
	}
	if gte != "2026-06-03T12:00:00" {
		t.Errorf("default --since should be 7 days back, got %q", gte)
	}
	if lte != "2026-06-10T12:00:00" {
		t.Errorf("default --until should be now, got %q", lte)
	}
}

func TestResolveTrafficRangeExplicit(t *testing.T) {
	now := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)

	cmd := NewListCmd()
	if err := cmd.Flags().Parse([]string{"--since", "2026-05-01", "--until", "2026-06-01"}); err != nil {
		t.Fatalf("flag parse error: %v", err)
	}

	gte, lte, err := resolveTrafficRange(cmd, now)
	if err != nil {
		t.Fatalf("resolveTrafficRange returned error: %v", err)
	}
	if gte != "2026-05-01T00:00:00" || lte != "2026-06-01T00:00:00" {
		t.Errorf("unexpected range: %q .. %q", gte, lte)
	}
}
