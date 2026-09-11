package objectstorage

import (
	"fmt"
	"net/url"
	"strings"
)

// DefaultSigningRegion is used when the endpoint hostname does not encode a
// region. VAST clusters accept any region string; Wasabi requires the one in
// the hostname, which SigningRegion extracts.
const DefaultSigningRegion = "us-east-1"

// SigningRegion derives the SigV4 signing region from a bucket endpoint.
//
// Latitude publishes two endpoint shapes (see the plan, §3):
//
//	https://s3.<region>.storage.sh       standard tier (CNAME to Wasabi) → <region>
//	https://objects.<site>.storage.sh    high_performance tier (VAST)     → <site>
//
// Wasabi rejects requests signed with the wrong region, so for `s3.*` hosts
// the second label is authoritative. VAST does not validate the region, so the
// site label is used only for consistency; any value would work there.
// Anything else falls back to DefaultSigningRegion.
func SigningRegion(endpoint string) string {
	host, _, err := EndpointHost(endpoint)
	if err != nil {
		return DefaultSigningRegion
	}
	labels := strings.Split(host, ".")
	if len(labels) >= 3 && (labels[0] == "s3" || labels[0] == "objects") && labels[1] != "" {
		return labels[1]
	}
	return DefaultSigningRegion
}

// EndpointHost splits an endpoint into the host[:port] minio expects and
// whether TLS should be used. Bare hosts (no scheme) default to https.
func EndpointHost(endpoint string) (host string, secure bool, err error) {
	e := strings.TrimSpace(endpoint)
	if e == "" {
		return "", false, fmt.Errorf("empty endpoint")
	}
	if !strings.Contains(e, "://") {
		e = "https://" + e
	}
	u, err := url.Parse(e)
	if err != nil || u.Host == "" {
		return "", false, fmt.Errorf("invalid endpoint %q", endpoint)
	}
	switch strings.ToLower(u.Scheme) {
	case "https":
		secure = true
	case "http":
		secure = false
	default:
		return "", false, fmt.Errorf("unsupported endpoint scheme %q (use http or https)", u.Scheme)
	}
	return u.Host, secure, nil
}
