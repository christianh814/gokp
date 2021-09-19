package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// createClusterCmd represents the createCluster command
var createClusterCmd = &cobra.Command{
	Use:     "create-cluster",
	Aliases: []string{"createCluster"},
	Short:   "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Grab the github token from the CLI
		ghToken, _ := cmd.Flags().GetString("github-token")
		clusterName, _ := cmd.Flags().GetString("cluster-name")
		privateRepo, _ := cmd.Flags().GetBool("private-repo")
		fmt.Println("createCluster called")

		fmt.Println("Your Token: ", ghToken)
		fmt.Println("Your Cluster Name: ", clusterName)
		fmt.Println("Private repo: ", privateRepo)
	},
}

func init() {
	rootCmd.AddCommand(createClusterCmd)
	createClusterCmd.Flags().String("github-token", "", "GitHub token to use.")
	createClusterCmd.Flags().String("cluster-name", "", "Name of your cluster.")
	createClusterCmd.Flags().BoolP("private-repo", "", false, "Create a private repo (default is \"false\")")

	// require the following flags
	createClusterCmd.MarkFlagRequired("github-token")
	createClusterCmd.MarkFlagRequired("cluster-name")

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// createClusterCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// createClusterCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
