package objectstorage

import (
	"testing"

	"github.com/spf13/pflag"
)

// The three examples from the aws user guide ("Use of Exclude and Include
// Filters"): include alone is a no-op, exclude-all then include narrows,
// order matters.
func TestFiltersAWSSemantics(t *testing.T) {
	var f Filters
	if !f.Include("anything.txt") {
		t.Fatal("no rules must include everything")
	}

	var onlyInclude Filters
	_ = onlyInclude.Add(true, "*.log")
	if !onlyInclude.Include("a.txt") {
		t.Error("--include alone must not exclude other files (aws semantics)")
	}

	var narrow Filters
	_ = narrow.Add(false, "*")
	_ = narrow.Add(true, "*.log")
	if narrow.Include("a.txt") || !narrow.Include("dir/b.log") {
		t.Error("--exclude '*' --include '*.log' must include only .log files")
	}

	var reversed Filters
	_ = reversed.Add(true, "*.log")
	_ = reversed.Add(false, "*")
	if reversed.Include("b.log") {
		t.Error("last matching rule wins: --include then --exclude '*' excludes everything")
	}

	var dirOnly Filters
	_ = dirOnly.Add(false, "*")
	_ = dirOnly.Add(true, "2026/09/*")
	if !dirOnly.Include("2026/09/dump.sql") || dirOnly.Include("2026/08/dump.sql") {
		t.Error("star must match across slashes like fnmatch in aws")
	}
}

func TestFilterFlagsPreserveOrder(t *testing.T) {
	var f Filters
	fs := pflag.NewFlagSet("t", pflag.ContinueOnError)
	FilterFlags(fs, &f)
	if err := fs.Parse([]string{"--exclude", "*", "--include", "*.log", "--exclude", "tmp/*"}); err != nil {
		t.Fatal(err)
	}
	if !f.Include("a.log") || f.Include("tmp/a.log") || f.Include("a.txt") {
		t.Errorf("order not preserved: %s", f.Describe())
	}
}

func TestGlobToRegexpInvalid(t *testing.T) {
	var f Filters
	if err := f.Add(true, "[a-"); err != nil {
		// An unterminated class is treated literally, not as an error.
		t.Errorf("unterminated class should be literal, got %v", err)
	}
}
