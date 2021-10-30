package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

// createClusterCmd represents the createCluster command
var createClusterCmd = &cobra.Command{
	Use:     "create-cluster",
	Aliases: []string{"createCluster"},
	Short:   "Create a GitOps Ready K8S Cluster",
	Long: `Create a GitOps Ready K8S Cluster using CAPI!

This is a PoC stage (proof of concept) and should NOT
be used for production. There will be lots of breaking changes
so beware. There be dragons here. PRE-PRE-ALPHA`,
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
	rootCmd.AddCommand(createClusterCmd)
}
