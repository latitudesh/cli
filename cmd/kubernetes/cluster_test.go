package kubernetes

import (
	"encoding/json"
	"testing"

	"github.com/latitudesh/latitudesh-go-sdk/models/components"
)

const clusterListFixture = `{
  "data": [
    {
      "id": "kc_abc",
      "type": "kubernetes_clusters",
      "attributes": {
        "name": "prod",
        "phase": "Provisioned",
        "ready": true,
        "created_at": "2026-06-01T12:00:00Z"
      }
    }
  ]
}`

const clusterGetFixture = `{
  "data": {
    "id": "kc_abc",
    "type": "kubernetes_clusters",
    "attributes": {
      "name": "prod",
      "phase": "Provisioned",
      "ready": true,
      "kubernetes_version": "v1.34.3+rke2r1",
      "location": "SAO2",
      "plan": "c2-small-x86",
      "worker_plan": "c2-small-x86",
      "control_plane_count": 3,
      "worker_count": 5,
      "control_plane_endpoint": "https://cp.example.com:6443"
    }
  }
}`

func TestClusterSummaryDecodingAndRow(t *testing.T) {
	var payload components.KubernetesClusters
	if err := json.Unmarshal([]byte(clusterListFixture), &payload); err != nil {
		t.Fatalf("could not unmarshal cluster list fixture: %v", err)
	}
	if len(payload.Data) != 1 {
		t.Fatalf("expected 1 cluster, got %d", len(payload.Data))
	}
	c := ClusterSummary{KubernetesClusterSummaryData: payload.Data[0]}
	row := c.TableRow()
	expectations := map[string]string{
		"id":    "kc_abc",
		"name":  "prod",
		"phase": "Provisioned",
		"ready": "yes",
	}
	for key, want := range expectations {
		if got := row[key].Value; got != want {
			t.Errorf("row[%q] = %q, want %q", key, got, want)
		}
	}
}

func TestClusterDecodingAndRow(t *testing.T) {
	var payload components.KubernetesCluster
	if err := json.Unmarshal([]byte(clusterGetFixture), &payload); err != nil {
		t.Fatalf("could not unmarshal cluster get fixture: %v", err)
	}
	if payload.Data == nil {
		t.Fatal("expected cluster data")
	}
	c := Cluster{KubernetesClusterData: *payload.Data}
	row := c.TableRow()
	expectations := map[string]string{
		"id":                     "kc_abc",
		"name":                   "prod",
		"phase":                  "Provisioned",
		"ready":                  "yes",
		"kubernetes_version":     "v1.34.3+rke2r1",
		"location":               "SAO2",
		"plan":                   "c2-small-x86",
		"worker_plan":            "c2-small-x86",
		"control_plane_count":    "3",
		"worker_count":           "5",
		"control_plane_endpoint": "https://cp.example.com:6443",
	}
	for key, want := range expectations {
		if got := row[key].Value; got != want {
			t.Errorf("row[%q] = %q, want %q", key, got, want)
		}
	}
}

func TestVersionRow(t *testing.T) {
	minor := "1.35"
	latest := "v1.35.3+rke2r1"
	v := Version{KubernetesAvailableVersionsData: components.KubernetesAvailableVersionsData{
		Minor:  &minor,
		Latest: &latest,
	}}
	row := v.TableRow()
	if row["minor"].Value != "1.35" {
		t.Errorf("minor = %q, want 1.35", row["minor"].Value)
	}
	if row["latest"].Value != "v1.35.3+rke2r1" {
		t.Errorf("latest = %q, want v1.35.3+rke2r1", row["latest"].Value)
	}
}
