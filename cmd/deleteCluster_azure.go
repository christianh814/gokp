package cmd

import (
	"os"

	"github.com/christianh814/gokp/cmd/capi"
	"github.com/christianh814/gokp/cmd/kind"
	"github.com/christianh814/gokp/cmd/utils"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// azureDeleteCmd represents the aws delete command
var azureDeleteCmd = &cobra.Command{
	Use:   "azure",
	Short: "Deletes a GOKP cluster running on Azure",
	Long: `This will delete your cluster that is running on Azure
based on the kubeconfig file and name you pass it.

This only deletes the cluster and not the git repo.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Create workdir and set variables
		WorkDir, _ = utils.CreateWorkDir()
		KindCfg = WorkDir + "/" + "kind.kubeconfig"
		tcpName := "gokp-bootstrapper"

		// cleanup workdir at the end
		defer os.RemoveAll(WorkDir)

		// Grab flags
		clusterName, _ := cmd.Flags().GetString("cluster-name")
		CapiCfg, _ := cmd.Flags().GetString("kubeconfig")

		// Create KIND cluster
		log.Info("Creating temporary control plane")
		err := kind.CreateKindCluster(tcpName, KindCfg)
		if err != nil {
			log.Fatal(err)
		}

		// Move Capi components to the KIND cluster
		log.Info("Moving CAPI Artifacts to the tempoary control plane")
		_, err = capi.MoveMgmtCluster(CapiCfg, KindCfg, "capz")
		if err != nil {
			log.Fatal(err)

		}

		// Delete cluster
		log.Info("Deleteing cluster: " + clusterName)
		_, err = capi.DeleteCluster(KindCfg, clusterName)
		if err != nil {
			log.Fatal(err)
		}

		// Delete local Kind Cluster
		log.Info("Deleting temporary control plane")
		err = kind.DeleteKindCluster(tcpName, KindCfg)
		if err != nil {
			log.Fatal(err)
		}

		// If we're here, the cluster should be deleted
		log.Info("Cluster " + clusterName + " successfully deleted")

	},
}

func init() {
	deleteClusterCmd.AddCommand(azureDeleteCmd)

	// Define flags for delete-cluster
	azureDeleteCmd.Flags().String("kubeconfig", "", "Path to the Kubeconfig file of the gokp cluster")
	azureDeleteCmd.Flags().String("cluster-name", "", "Name of the gokp cluster.")

	// all flags required
	azureDeleteCmd.MarkFlagRequired("kubeconfig")
	azureDeleteCmd.MarkFlagRequired("cluster-name")

}
