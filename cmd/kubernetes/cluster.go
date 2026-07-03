package kubernetes

import (
	"strconv"

	"github.com/latitudesh/latitudesh-go-sdk/models/components"
	"github.com/latitudesh/lsh/internal/output/table"
	"github.com/latitudesh/lsh/internal/renderer"
)

func getStr(s *string) string {
	if s != nil {
		return *s
	}
	return ""
}

// ClusterList is a renderable collection of clusters (list/summary shape).
type ClusterList struct {
	Data []*ClusterSummary
}

func (m *ClusterList) GetData() []renderer.ResponseData {
	var data []renderer.ResponseData
	for _, v := range m.Data {
		data = append(data, v)
	}
	return data
}

// ClusterSummary wraps the SDK summary data returned by the list endpoint.
type ClusterSummary struct {
	components.KubernetesClusterSummaryData
}

func (m *ClusterSummary) GetData() []renderer.ResponseData {
	return []renderer.ResponseData{m}
}

func (m *ClusterSummary) TableRow() table.Row {
	var name, phase, createdAt string
	ready := ""
	if attr := m.Attributes; attr != nil {
		name = getStr(attr.Name)
		if attr.Phase != nil {
			phase = string(*attr.Phase)
		}
		if attr.Ready != nil && *attr.Ready {
			ready = "yes"
		} else if attr.Ready != nil {
			ready = "no"
		}
		if attr.CreatedAt != nil {
			createdAt = attr.CreatedAt.String()
		}
	}

	return table.Row{
		"id":         table.Cell{Label: "ID", Value: table.String(getStr(m.ID))},
		"name":       table.Cell{Label: "Name", Value: table.String(name)},
		"phase":      table.Cell{Label: "Phase", Value: table.String(phase)},
		"ready":      table.Cell{Label: "Ready", Value: table.String(ready)},
		"created_at": table.Cell{Label: "Created At", Value: table.String(createdAt)},
	}
}

// Cluster wraps the full SDK cluster data returned by the get endpoint.
type Cluster struct {
	components.KubernetesClusterData
}

func (m *Cluster) GetData() []renderer.ResponseData {
	return []renderer.ResponseData{m}
}

func (m *Cluster) TableRow() table.Row {
	var (
		name, phase, version, location, plan, workerPlan, endpoint, createdAt string
		controlPlaneCount, workerCount                                        int64
	)
	ready := ""

	if attr := m.Attributes; attr != nil {
		name = getStr(attr.Name)
		if attr.Phase != nil {
			phase = string(*attr.Phase)
		}
		version = getStr(attr.KubernetesVersion)
		location = getStr(attr.Location)
		plan = getStr(attr.Plan)
		workerPlan = getStr(attr.WorkerPlan)
		endpoint = getStr(attr.ControlPlaneEndpoint)
		if attr.ControlPlaneCount != nil {
			controlPlaneCount = *attr.ControlPlaneCount
		}
		if attr.WorkerCount != nil {
			workerCount = *attr.WorkerCount
		}
		if attr.Ready != nil && *attr.Ready {
			ready = "yes"
		} else if attr.Ready != nil {
			ready = "no"
		}
		if attr.CreatedAt != nil {
			createdAt = attr.CreatedAt.String()
		}
	}

	return table.Row{
		"id":                     table.Cell{Label: "ID", Value: table.String(getStr(m.ID))},
		"name":                   table.Cell{Label: "Name", Value: table.String(name)},
		"phase":                  table.Cell{Label: "Phase", Value: table.String(phase)},
		"ready":                  table.Cell{Label: "Ready", Value: table.String(ready)},
		"kubernetes_version":     table.Cell{Label: "Version", Value: table.String(version)},
		"location":               table.Cell{Label: "Location", Value: table.String(location)},
		"plan":                   table.Cell{Label: "Plan", Value: table.String(plan)},
		"worker_plan":            table.Cell{Label: "Worker Plan", Value: table.String(workerPlan)},
		"control_plane_count":    table.Cell{Label: "Control Plane", Value: table.String(strconv.FormatInt(controlPlaneCount, 10))},
		"worker_count":           table.Cell{Label: "Workers", Value: table.String(strconv.FormatInt(workerCount, 10))},
		"control_plane_endpoint": table.Cell{Label: "Endpoint", Value: table.String(endpoint)},
		"created_at":             table.Cell{Label: "Created At", Value: table.String(createdAt)},
	}
}
