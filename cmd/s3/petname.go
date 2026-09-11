package s3

import (
	"crypto/rand"
	"math/big"

	"github.com/latitudesh/lsh/internal/objectstorage"
)

// Access key names follow the same "pet name" pattern the dashboard uses when
// it auto-suggests a name: key-<adjective>-<noun>-<tier>, e.g. key-brave-otter-std.
// There is no random numeric segment; uniqueness is enforced per project, so a
// candidate that collides is simply re-rolled (see createAccessKeyRetrying).
// The names satisfy the API rules (lowercase, 3-25 chars, start/end alphanumeric).

var petAdjectives = []string{
	"amber", "azure", "bold", "brave", "brisk", "calm", "clever", "cosmic",
	"crimson", "daring", "deep", "eager", "electric", "gentle", "golden", "happy",
	"jolly", "keen", "lively", "lucky", "mellow", "merry", "mighty", "noble",
	"plum", "proud", "quiet", "rapid", "royal", "shiny", "silent", "sleek",
	"smart", "snappy", "solar", "spry", "stellar", "sturdy", "sunny", "swift",
	"teal", "tidy", "vivid", "warm", "wise", "witty", "young", "zesty",
}

var petNouns = []string{
	"otter", "falcon", "harbor", "forest", "river", "meadow", "comet", "canyon",
	"maple", "willow", "cedar", "cobra", "lynx", "heron", "raven", "badger",
	"marlin", "puma", "bison", "koala", "gecko", "panda", "tiger", "walrus",
	"zephyr", "summit", "delta", "ember", "glacier", "harvest", "island", "jungle",
	"lagoon", "monsoon", "nebula", "orchid", "pebble", "quartz", "ridge", "sequoia",
	"tundra", "vortex", "wave", "beacon", "boulder", "cascade", "dune", "fjord",
}

// tierAbbr maps a storage class to the dashboard's short suffix.
func tierAbbr(storageClass string) string {
	if storageClass == objectstorage.ClassHighPerformance {
		return "hp"
	}
	return "std"
}

// generateKeyName returns a fresh pet name for the given storage class, capped
// at the API's 25-character limit.
func generateKeyName(storageClass string) string {
	return capKeyName("key-" + pick(petAdjectives) + "-" + pick(petNouns) + "-" + tierAbbr(storageClass))
}

// pick returns a uniformly random element of list using crypto/rand, falling
// back to the first element only if the source ever fails.
func pick(list []string) string {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(len(list))))
	if err != nil {
		return list[0]
	}
	return list[n.Int64()]
}
