package kubernetes

import (
	"testing"

	"github.com/latitudesh/latitudesh-go-sdk/models/components"
)

func TestBuildCreateRequest(t *testing.T) {
	cmd := NewCreateCmd()
	if err := cmd.Flags().Parse([]string{
		"--project", "my-project",
		"--region", "SAO2",
		"--plan", "c2-small-x86",
		"--name", "my-cluster",
		"--worker-plan", "c2-small-x86",
		"--ssh-keys", "key_a",
		"--kubernetes-version", "v1.34.3+rke2r1",
		"--control-plane-count", "3",
		"--worker-count", "5",
		"--os", "ubuntu_24_04_x64_lts",
	}); err != nil {
		t.Fatalf("flag parse error: %v", err)
	}

	req, err := buildCreateRequest(cmd)
	if err != nil {
		t.Fatalf("buildCreateRequest returned error: %v", err)
	}
	attrs := req.Data.Attributes
	if attrs.ProjectID != "my-project" {
		t.Errorf("ProjectID = %q, want my-project", attrs.ProjectID)
	}
	if attrs.Site != "SAO2" {
		t.Errorf("Site = %q, want SAO2", attrs.Site)
	}
	if attrs.Plan != "c2-small-x86" {
		t.Errorf("Plan = %q, want c2-small-x86", attrs.Plan)
	}
	if attrs.Name == nil || *attrs.Name != "my-cluster" {
		t.Errorf("Name = %v, want my-cluster", attrs.Name)
	}
	if attrs.WorkerPlan == nil || *attrs.WorkerPlan != "c2-small-x86" {
		t.Errorf("WorkerPlan = %v, want c2-small-x86", attrs.WorkerPlan)
	}
	if len(attrs.SSHKeys) != 1 || attrs.SSHKeys[0] != "key_a" {
		t.Errorf("SSHKeys = %v, want [key_a]", attrs.SSHKeys)
	}
	if attrs.KubernetesVersion == nil || *attrs.KubernetesVersion != "v1.34.3+rke2r1" {
		t.Errorf("KubernetesVersion = %v, want v1.34.3+rke2r1", attrs.KubernetesVersion)
	}
	if attrs.ControlPlaneCount == nil || *attrs.ControlPlaneCount != 3 {
		t.Errorf("ControlPlaneCount = %v, want 3", attrs.ControlPlaneCount)
	}
	if attrs.WorkerCount == nil || *attrs.WorkerCount != 5 {
		t.Errorf("WorkerCount = %v, want 5", attrs.WorkerCount)
	}
	if req.Data.Type != components.CreateKubernetesClusterTypeKubernetesClusters {
		t.Errorf("Type = %v, want kubernetes_clusters", req.Data.Type)
	}
}

func TestBuildCreateRequestRequiredFields(t *testing.T) {
	cases := [][]string{
		{"--region", "SAO2", "--plan", "c2-small-x86"},        // missing project
		{"--project", "my-project", "--plan", "c2-small-x86"}, // missing region
		{"--project", "my-project", "--region", "SAO2"},       // missing plan
	}
	for _, args := range cases {
		cmd := NewCreateCmd()
		if err := cmd.Flags().Parse(args); err != nil {
			t.Fatalf("flag parse error: %v", err)
		}
		if _, err := buildCreateRequest(cmd); err == nil {
			t.Errorf("expected error for args %v, got nil", args)
		}
	}
}
