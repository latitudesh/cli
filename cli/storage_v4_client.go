package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// v4APIVersion is sent as the `Api-Version` header on every v4 request. The
// API selects routes based on this header; v3 routes (legacy backend) keep
// serving clients that omit the header.
const (
	v4APIVersion     = "2026-05-11"
	v4DefaultBaseURL = "https://api.latitude.sh"
	v4Timeout        = 30 * time.Second
)

// V4VolumeAttributes is the v4 mount/show response shape. Fields are pointers
// where they are absent on the v3 backend; the CLI requires the connection
// fields to be populated and errors out otherwise.
type V4VolumeAttributes struct {
	Name              string   `json:"name"`
	SizeInGB          int      `json:"size_in_gb"`
	CreatedAt         string   `json:"created_at"`
	Region            string   `json:"region,omitempty"`
	SubsystemNQN      string   `json:"subsystem_nqn,omitempty"`
	NamespaceID       *int     `json:"namespace_id,omitempty"`
	GatewayVIPs       []string `json:"gateway_vips,omitempty"`
	DiscoveryEndpoint string   `json:"discovery_endpoint,omitempty"`
}

type V4Volume struct {
	ID         string             `json:"id"`
	Type       string             `json:"type"`
	Attributes V4VolumeAttributes `json:"attributes"`
}

type v4Envelope struct {
	Data V4Volume `json:"data"`
}

type v4Client struct {
	apiKey  string
	baseURL string
	http    *http.Client
}

func newV4Client(apiKey string) *v4Client {
	baseURL := os.Getenv("LSH_API_URL")
	if baseURL == "" {
		baseURL = v4DefaultBaseURL
	}
	return &v4Client{
		apiKey:  apiKey,
		baseURL: baseURL,
		http:    &http.Client{Timeout: v4Timeout},
	}
}

func (c *v4Client) do(method, path string, body []byte) ([]byte, int, error) {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, c.baseURL+path, reader)
	if err != nil {
		return nil, 0, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Api-Version", v4APIVersion)
	req.Header.Set("Accept", "application/vnd.api+json")
	if body != nil {
		req.Header.Set("Content-Type", "application/vnd.api+json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("execute request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("read response: %w", err)
	}
	return data, resp.StatusCode, nil
}

func (c *v4Client) decodeVolume(data []byte) (*V4Volume, error) {
	var env v4Envelope
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, fmt.Errorf("decode volume response: %w", err)
	}
	return &env.Data, nil
}

// GetVolume fetches one volume by ID via the v4 routes.
func (c *v4Client) GetVolume(id string) (*V4Volume, error) {
	data, status, err := c.do("GET", "/storage/volumes/"+id, nil)
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, fmt.Errorf("GET /storage/volumes/%s returned %d: %s", id, status, string(data))
	}
	return c.decodeVolume(data)
}

// MountVolume authorizes the client's NQN on the storage subsystem and
// returns the connection info (subsystem NQN, namespace ID, gateway VIPs)
// that the host needs to establish the NVMe-oF/TCP path.
func (c *v4Client) MountVolume(id, clientNQN string) (*V4Volume, error) {
	body, err := json.Marshal(map[string]any{
		"data": map[string]any{
			"type": "volumes",
			"attributes": map[string]any{
				"nqn": clientNQN,
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("encode mount request: %w", err)
	}

	data, status, err := c.do("POST", "/storage/volumes/"+id+"/mount", body)
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, fmt.Errorf("POST /storage/volumes/%s/mount returned %d: %s", id, status, string(data))
	}
	return c.decodeVolume(data)
}
