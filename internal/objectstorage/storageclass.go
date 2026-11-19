package objectstorage

import (
	"fmt"
	"strings"

	"github.com/latitudesh/lsh/internal/exitcode"
)

// ParseStorageClass normalizes a user-supplied storage class. It is the single
// alias table shared by every command so the same spelling is accepted
// everywhere: standard|std|wasabi and high_performance|high-performance|hp|
// high|vast (case-insensitive). An empty value returns "" without error so
// callers can apply their own default.
func ParseStorageClass(s string) (string, error) {
	v := strings.ToLower(strings.TrimSpace(s))
	switch v {
	case "":
		return "", nil
	case ClassStandard, "std", "wasabi":
		return ClassStandard, nil
	case ClassHighPerformance, "high-performance", "highperformance", "hp", "high", "vast":
		return ClassHighPerformance, nil
	}
	return "", exitcode.Errorf(exitcode.Usage, "invalid storage class %q (expected standard or high_performance)", s)
}

// StorageClassLabel returns a short human label for a storage class.
func StorageClassLabel(class string) string {
	switch class {
	case ClassHighPerformance:
		return "high_performance (VAST)"
	case ClassStandard:
		return "standard (Wasabi)"
	}
	return class
}

var _ = fmt.Sprintf
