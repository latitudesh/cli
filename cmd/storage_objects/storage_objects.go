package storage_objects

import (
	"github.com/latitudesh/latitudesh-go-sdk/models/components"
	"github.com/latitudesh/lsh/internal/output/table"
	"github.com/latitudesh/lsh/internal/renderer"
)

// Buckets is the renderable collection of object storage buckets.
type Buckets struct {
	Data []*Bucket
}

func (m *Buckets) GetData() []renderer.ResponseData {
	var data []renderer.ResponseData
	for _, v := range m.Data {
		data = append(data, v)
	}
	return data
}

// Bucket wraps the SDK ObjectStorageData so it can be rendered by the shared
// renderer (table / -o json,yaml / --query).
type Bucket struct {
	components.ObjectStorageData
}

func (m *Bucket) GetData() []renderer.ResponseData {
	return []renderer.ResponseData{m}
}

func (m *Bucket) TableRow() table.Row {
	b := m.ObjectStorageData

	getStr := func(s *string) string {
		if s != nil {
			return *s
		}
		return ""
	}

	var name, bucketName, region, storageClass, endpoint, createdAt string
	if attr := b.Attributes; attr != nil {
		name = getStr(attr.Name)
		bucketName = getStr(attr.BucketName)
		endpoint = getStr(attr.Endpoint)
		if attr.StorageClass != nil {
			storageClass = string(*attr.StorageClass)
		}
		// The API sends region as {city, country} on get (id is not populated
		// and list omits the attribute entirely); prefer the most specific field.
		if attr.Region != nil {
			region = getStr(attr.Region.ID)
			if region == "" {
				region = getStr(attr.Region.City)
			}
			if region == "" {
				region = getStr(attr.Region.Country)
			}
		}
		if attr.CreatedAt != nil {
			createdAt = attr.CreatedAt.Format("2006-01-02T15:04:05Z07:00")
		}
	}

	return table.Row{
		"id": table.Cell{
			Label: "ID",
			Value: table.String(getStr(b.ID)),
		},
		"name": table.Cell{
			Label: "Name",
			Value: table.String(name),
		},
		"bucket_name": table.Cell{
			Label: "Bucket Name",
			Value: table.String(bucketName),
		},
		"region": table.Cell{
			Label: "Region",
			Value: table.String(region),
		},
		"storage_class": table.Cell{
			Label: "Storage Class",
			Value: table.String(storageClass),
		},
		"endpoint": table.Cell{
			Label: "Endpoint",
			Value: table.String(endpoint),
		},
		"created_at": table.Cell{
			Label: "Created At",
			Value: table.String(createdAt),
		},
	}
}
