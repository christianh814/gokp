package cmd

import (
	"os"

	log "github.com/sirupsen/logrus"

	"github.com/christianh814/gokp/cmd/capi"
	"github.com/christianh814/gokp/cmd/kind"
	"github.com/christianh814/gokp/cmd/utils"
	"github.com/spf13/cobra"
)

// deleteClusterCmd represents the deleteCluster command
var deleteClusterCmd = &cobra.Command{
	Use:     "delete-cluster",
	Aliases: []string{"deleteCluster"},
	Short:   "Deletes a gokp cluster",
	Long: `This will delete your cluster based on the kubeconfig file
and name you pass it. This only deletes the cluster and not the git repo.`,
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
		_, err = capi.MoveMgmtCluster(CapiCfg, KindCfg)
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
		log.Info("Cluster " + clusterName + "successfully deleted")

	},
}

func init() {
	rootCmd.AddCommand(deleteClusterCmd)

	// Define flags for delete-cluster
	deleteClusterCmd.Flags().String("kubeconfig", "", "Path to the Kubeconfig file of the gokp cluster")
	deleteClusterCmd.Flags().String("cluster-name", "", "Name of the gokp cluster.")

	// all flags required
	deleteClusterCmd.MarkFlagRequired("kubeconfig")
	deleteClusterCmd.MarkFlagRequired("cluster-name")

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// deleteClusterCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// deleteClusterCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
