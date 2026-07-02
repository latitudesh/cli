package prompt

import (
	"context"
	"errors"
	"fmt"

	"github.com/latitudesh/lsh/internal/authclient"
	"github.com/latitudesh/lsh/internal/tui"
)

// AllProjectsSentinel is returned by SelectProject when the user
// picks the "All projects" entry. Commands that support listing
// across projects should treat this as "skip the --project filter".
const AllProjectsSentinel = "__all_projects__"

// ProjectPicker abstracts the project-listing call so callers don't
// need to thread http clients through the prompt API.
type ProjectPicker interface {
	ListProjects(ctx context.Context, token string) ([]authclient.Project, error)
}

// SelectProject lists the team's projects and prompts the user to pick
// one interactively. Returns the picked project's id_hash, or
// AllProjectsSentinel if the user chose "All projects".
//
// The "All projects" entry is appended only when allowAll is true —
// it makes sense for list commands but not for commands that require
// a single project (e.g. servers create).
func SelectProject(ctx context.Context, client ProjectPicker, token string, allowAll bool) (string, error) {
	if token == "" {
		return "", errors.New("not logged in — run `lsh login` first")
	}
	projects, err := client.ListProjects(ctx, token)
	if err != nil {
		return "", fmt.Errorf("could not list projects: %w", err)
	}
	if len(projects) == 0 {
		return "", errors.New("no projects found for the active team")
	}

	items := make([]string, 0, len(projects)+1)
	descriptions := make([]string, 0, len(projects)+1)
	for _, p := range projects {
		items = append(items, p.Slug)
		descriptions = append(descriptions, fmt.Sprintf("%s — %s", p.Name, p.ID))
	}
	if allowAll {
		items = append(items, "All projects")
		descriptions = append(descriptions, "Run across every project in this team")
	}

	choice, err := tui.RunList("Select a project", items, descriptions)
	if err != nil {
		return "", err
	}
	// Match a real project first, so a project that happens to be named
	// "All projects" wins over the sentinel entry rather than being
	// silently treated as "skip the filter".
	for _, p := range projects {
		if p.Slug == choice {
			return p.ID, nil
		}
	}
	if allowAll && choice == "All projects" {
		return AllProjectsSentinel, nil
	}
	return "", fmt.Errorf("unexpected selection: %s", choice)
}
