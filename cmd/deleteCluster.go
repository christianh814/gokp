package cmd

import (
	"os"

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
		// Show help if a subcommand isn't supplied
		if len(args) == 0 {
			cmd.Help()
			os.Exit(0)
		}
		whatToRun := args[0]
		switch {
		case whatToRun != "help":
			cmd.Help()
		case whatToRun != "aws":
			cmd.Help()
			os.Exit(0)
		case whatToRun != "development":
			cmd.Help()
			os.Exit(0)
		}

	},
}

func init() {
	rootCmd.AddCommand(deleteClusterCmd)
}
