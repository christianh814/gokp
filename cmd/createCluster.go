package cmd

import (
	"github.com/christianh814/project-spichern/cmd/github"
	"github.com/christianh814/project-spichern/cmd/kind"
	"github.com/christianh814/project-spichern/cmd/templates"
	"github.com/christianh814/project-spichern/cmd/utils"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// createClusterCmd represents the createCluster command
var createClusterCmd = &cobra.Command{
	Use:     "create-cluster",
	Aliases: []string{"createCluster"},
	Short:   "Create a GitOps Ready K8S Cluster",
	Long: `Create a GitOps Ready K8S Cluster using
KIND + CAPI + Argo CD!

Currenly only AWS with GitHub works.

This is a PoC stage (proof of concept) and should NOT
be used for production. There will be lots of breaking changes
so beware. There be dragons here.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Grab repo related flags
		ghToken, _ := cmd.Flags().GetString("github-token")
		clusterName, _ := cmd.Flags().GetString("cluster-name")
		privateRepo, _ := cmd.Flags().GetBool("private-repo")

		// Grab AWS related flags
		awsRegion, _ := cmd.Flags().GetString("aws-region")
		awsAccessKey, _ := cmd.Flags().GetString("aws-access-key")
		awsSecretKey, _ := cmd.Flags().GetString("aws-secret-key")
		awsSSHKey, _ := cmd.Flags().GetString("aws-ssh-key")
		awsCPMachine, _ := cmd.Flags().GetString("aws-control-plane-machine")
		awsWMachine, _ := cmd.Flags().GetString("aws-node-machine")

		// Run PreReq Checks
		_, err := utils.CheckPreReqs()
		if err != nil {
			log.Fatal(err)
		}

		// Create KIND instance
		log.Info("Creating KIND instance")
		err = kind.CreateKindCluster("gokp-boostrapper", KindCfg)

		if err != nil {
			log.Fatal(err)
		}

		// Create CAPI instance on AWS
		log.Info(awsRegion)
		log.Info(awsAccessKey)
		log.Info(awsSecretKey)
		log.Info(awsSSHKey)
		log.Info(awsCPMachine)
		log.Info(awsWMachine)

		// Create the GitOps repo

		_, err = github.CreateRepo(&clusterName, ghToken, &privateRepo, WorkDir)
		if err != nil {
			log.Fatal(err)
		}

		// Create repo dir structure. Including Argo CD Yamls
		_, err = templates.CreateRepoSkel(&clusterName, WorkDir)
		if err != nil {
			log.Fatal(err)
		}

		// Export/Create Cluster YAML to the Repo

		// Make sure kustomize is used

		// Git push repo

		// Install Argo CD on the newly created cluster

		// Deploy applications/applicationsets

	},
}

func init() {
	rootCmd.AddCommand(createClusterCmd)
	// Repo specific flags
	createClusterCmd.Flags().String("github-token", "", "GitHub token to use.")
	createClusterCmd.Flags().String("cluster-name", "", "Name of your cluster.")
	createClusterCmd.Flags().BoolP("private-repo", "", false, "Create a private repo.")

	//AWS Specific flags
	createClusterCmd.Flags().String("aws-region", "us-east-1", "Which region to deploy to.")
	createClusterCmd.Flags().String("aws-access-key", "", "Your AWS Access Key.")
	createClusterCmd.Flags().String("aws-secret-key", "", "Your AWS Secret Key.")
	createClusterCmd.Flags().String("aws-ssh-key", "default", "The SSH key in AWS that you want to use for the instances.")
	createClusterCmd.Flags().String("aws-control-plane-machine", "m4.xlarge", "The AWS instance type for the Control Plane")
	createClusterCmd.Flags().String("aws-node-machine", "m4.xlarge", "The AWS instance type for the Worker instances")

	// require the following flags
	createClusterCmd.MarkFlagRequired("github-token")
	createClusterCmd.MarkFlagRequired("cluster-name")
	createClusterCmd.MarkFlagRequired("aws-access-key")
	createClusterCmd.MarkFlagRequired("aws-secret-key")

	// Vars that get set at Runtime
	WorkDir, _ = utils.CreateWorkDir()
	KindCfg = WorkDir + "/" + "kind.kubeconfig"
	CapiCfg = WorkDir + "/" + "capi.kubeconfig"
	// commenting out for now for testing
	// defer os.RemoveAll(Workdir)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// createClusterCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// createClusterCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
