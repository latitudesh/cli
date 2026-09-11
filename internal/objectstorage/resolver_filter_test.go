package objectstorage

import (
	"context"
	"testing"

	"github.com/latitudesh/latitudesh-go-sdk/models/components"
	"github.com/latitudesh/lsh/internal/exitcode"
)

func classData(id, name, class string) components.ObjectStorageData {
	i, n := id, name
	c := components.StorageClass(class)
	return components.ObjectStorageData{ID: &i, Attributes: &components.ObjectStorageDataAttributes{Name: &n, StorageClass: &c}}
}

func TestApplyFiltersByClass(t *testing.T) {
	matches := []components.ObjectStorageData{
		classData("bkt_std", "backups", "standard"),
		classData("bkt_hp", "backups", "high_performance"),
	}
	r := &Resolver{ClassFilter: "high_performance"}
	out, err := r.applyFilters(context.Background(), matches)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || *out[0].ID != "bkt_hp" {
		t.Fatalf("class filter kept %d matches, want just bkt_hp", len(out))
	}
	// No filter keeps everything.
	out, _ = (&Resolver{}).applyFilters(context.Background(), matches)
	if len(out) != 2 {
		t.Fatalf("no filter must keep both, got %d", len(out))
	}
}

func TestResolveFilterErr(t *testing.T) {
	r := &Resolver{FilterErr: exitcode.Errorf(exitcode.Usage, "bad class")}
	if _, err := r.Resolve(context.Background(), "backups"); exitcode.Of(err) != exitcode.Usage {
		t.Fatalf("Resolve must return FilterErr, got %v", err)
	}
	if _, err := r.ListBuckets(context.Background()); exitcode.Of(err) != exitcode.Usage {
		t.Fatalf("ListBuckets must return FilterErr, got %v", err)
	}
}
