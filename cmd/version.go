package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Displays version.",
	Long:  `This command will display the version of the CLI in json format`,
	Run: func(cmd *cobra.Command, args []string) {
		versionMap := map[string]string{"gokp": rootCmd.Version}
		versionJson, _ := json.Marshal(versionMap)
		fmt.Println(string(versionJson))
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// versionCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// versionCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
