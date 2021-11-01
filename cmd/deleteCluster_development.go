package cmd

import (
	"os"

	"github.com/christianh814/gokp/cmd/kind"
	"github.com/christianh814/gokp/cmd/utils"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// developmentDeleteCmd represents the developmentDelete command
var developmentDeleteCmd = &cobra.Command{
	Use:   "development",
	Short: "Deletes the gokp development cluster",
	Long: `This will delete your development cluster based on the kubeconfig file
and name you pass it. This only deletes the local development cluster and not the git repo.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Create workdir and set variables
		WorkDir, _ = utils.CreateWorkDir()
		defer os.RemoveAll(WorkDir)

		// Grab flags
		clusterName, _ := cmd.Flags().GetString("cluster-name")
		CapiCfg, _ := cmd.Flags().GetString("kubeconfig")

		// Delete local Kind Cluster
		log.Info("Deleting development cluster " + clusterName)
		err := kind.DeleteKindCluster(clusterName, CapiCfg)
		if err != nil {
			log.Fatal(err)
		}

		// If we're here, the cluster should be deleted
		log.Info("Cluster " + clusterName + " successfully deleted")
	},
}

func init() {
	deleteClusterCmd.AddCommand(developmentDeleteCmd)

	// Define flags for delete-cluster
	developmentDeleteCmd.Flags().String("kubeconfig", "", "Path to the Kubeconfig file of the gokp cluster")
	developmentDeleteCmd.Flags().String("cluster-name", "", "Name of the gokp cluster.")

	// all flags required
	developmentDeleteCmd.MarkFlagRequired("kubeconfig")
	developmentDeleteCmd.MarkFlagRequired("cluster-name")
}
