package cmd

import (
	kubernetes "github.com/latitudesh/lsh/cmd/kubernetes"
	cobra "github.com/spf13/cobra"
)

func init() {
	kubernetesCmd.AddCommand(kubernetes.NewClustersCmd())
	kubernetesCmd.AddCommand(kubernetes.NewVersionsCmd())
	kubernetesCmd.AddCommand(kubernetes.NewKubeconfigCmd())

	rootCmd.AddCommand(kubernetesCmd)
}

var kubernetesCmd = &cobra.Command{
	Use:     "kubernetes",
	Aliases: []string{"k8s"},
	Short:   "Manage Kubernetes clusters",
	Long: "Manage the team's Kubernetes clusters: list, inspect, create, and delete\n" +
		"clusters, list available versions, and fetch a cluster's kubeconfig.",
	Example: `  lsh kubernetes clusters list --project my-project
  lsh kubernetes clusters create --project my-project --region SAO2 --plan c2-small-x86 --wait
  lsh kubernetes versions list
  lsh kubernetes kubeconfig kc_xxxxxxxx > kubeconfig.yaml`,
}
