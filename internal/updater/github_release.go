package updater

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// GithubRelease represents a release object from the Github API
type GithubRelease struct {
	Id      int    `json:"id"`
	TagName string `json:"tag_name"`
}

// latestLshRelease returns the latest lsh CLI release
func LatestLshRelease() (*GithubRelease, error) {
	release := GithubRelease{}
	err := buildRelease("repos/latitudesh/lsh/releases/latest", &release)
	if err != nil {
		return nil, err
	}
	return &release, err
}

// buildRelease Unmarshals github information into a release object
func buildRelease(url string, release *GithubRelease) error {
	resp, err := gitHubRequest("GET", url, &http.Client{})
	if err != nil {
		return err
	}
	err = json.Unmarshal([]byte(resp), release)
	if err != nil {
		return err
	}
	return nil
}

// gitHubRequest accepts a method, path and a client to execute a github request
func gitHubRequest(method, path string, client *http.Client) (string, error) {
	req, _ := http.NewRequest(method, fmt.Sprintf("https://api.github.com/%s", path), nil)
	req.Header = http.Header{
		"Accept":               {"application/vnd.github+json"},
		"X-GitHub-Api-Version": {"2022-11-28"},
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API %s: unexpected HTTP status %s", path, resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}
