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

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// createClusterCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// createClusterCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
