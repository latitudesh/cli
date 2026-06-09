package cli

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// newProjectCmd builds a command with the flags resolveProjectFlag inspects.
func newProjectCmd(withAllProjects bool) *cobra.Command {
	cmd := &cobra.Command{Use: "x"}
	cmd.Flags().String("project", "", "")
	if withAllProjects {
		cmd.Flags().Bool("all-projects", false, "")
	}
	return cmd
}

// forceNonInteractive makes resolveProjectFlag take the deterministic
// (non-prompt) path for the duration of the test.
func forceNonInteractive(t *testing.T) {
	t.Helper()
	prev := isInteractive
	isInteractive = func() bool { return false }
	t.Cleanup(func() { isInteractive = prev })
}

func TestResolveProjectFlag_NoProjectFlag_NoOp(t *testing.T) {
	cmd := &cobra.Command{Use: "x"} // no --project flag at all
	if err := resolveProjectFlag(cmd); err != nil {
		t.Fatalf("expected no-op for command without --project, got %v", err)
	}
}

func TestResolveProjectFlag_ExplicitProject_Passes(t *testing.T) {
	cmd := newProjectCmd(true)
	_ = cmd.Flags().Set("project", "proj_123")
	if err := resolveProjectFlag(cmd); err != nil {
		t.Fatalf("expected pass with explicit --project, got %v", err)
	}
}

func TestResolveProjectFlag_EnvProject_Passes(t *testing.T) {
	t.Setenv("LSH_PROJECT", "proj_env")
	cmd := newProjectCmd(true)
	if err := resolveProjectFlag(cmd); err != nil {
		t.Fatalf("expected pass with LSH_PROJECT, got %v", err)
	}
	if v, _ := cmd.Flags().GetString("project"); v != "proj_env" {
		t.Fatalf("expected project set from env, got %q", v)
	}
}

func TestResolveProjectFlag_AllProjectsTrue_Skips(t *testing.T) {
	cmd := newProjectCmd(true)
	_ = cmd.Flags().Set("all-projects", "true")
	if err := resolveProjectFlag(cmd); err != nil {
		t.Fatalf("expected skip with --all-projects=true, got %v", err)
	}
}

func TestResolveProjectFlag_AllProjectsFalse_DoesNotBypass(t *testing.T) {
	forceNonInteractive(t)
	cmd := newProjectCmd(true)
	_ = cmd.Flags().Set("all-projects", "false")
	err := resolveProjectFlag(cmd)
	if err == nil {
		t.Fatal("--all-projects=false must not bypass the project requirement")
	}
	if !strings.Contains(err.Error(), "--all-projects") {
		t.Fatalf("hint should mention --all-projects on a command that has it, got %q", err.Error())
	}
}

func TestResolveProjectFlag_NoInputForcesErrorEvenOnTTY(t *testing.T) {
	// Force interactive (TTY) so only --no-input can trigger the error path.
	prev := isInteractive
	isInteractive = func() bool { return true }
	t.Cleanup(func() { isInteractive = prev })

	cmd := newProjectCmd(true)
	cmd.Flags().Bool("no-input", false, "")
	_ = cmd.Flags().Set("no-input", "true")

	if err := resolveProjectFlag(cmd); err == nil {
		t.Fatal("--no-input must force the project-required error even on a TTY")
	}
}

func TestResolveProjectFlag_NonInteractiveHintOmitsAllProjectsWhenUnsupported(t *testing.T) {
	forceNonInteractive(t)
	cmd := newProjectCmd(false) // no --all-projects flag
	err := resolveProjectFlag(cmd)
	if err == nil {
		t.Fatal("expected project-required error")
	}
	if strings.Contains(err.Error(), "--all-projects") {
		t.Fatalf("hint must not mention --all-projects when the command lacks it, got %q", err.Error())
	}
}
