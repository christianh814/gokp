package cmd

import (
	"os"

	"github.com/christianh814/gokp/cmd/argo"
	"github.com/christianh814/gokp/cmd/capi"
	"github.com/christianh814/gokp/cmd/export"

	"github.com/christianh814/gokp/cmd/github"
	"github.com/christianh814/gokp/cmd/kind"
	"github.com/christianh814/gokp/cmd/templates"
	"github.com/christianh814/gokp/cmd/utils"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// createClusterCmd represents the createCluster command
var createClusterCmd = &cobra.Command{
	Use:     "create-cluster",
	Aliases: []string{"createCluster"},
	Short:   "Create a GitOps Ready K8S Cluster",
	Long: `Create a GitOps Ready K8S Cluster using CAPI + Argo CD!

Currenly only AWS + GitHub works.

This is a PoC stage (proof of concept) and should NOT
be used for production. There will be lots of breaking changes
so beware. There be dragons here. PRE-PRE-ALPHA`,
	Run: func(cmd *cobra.Command, args []string) {
		// Create workdir and set variables based on that
		WorkDir, _ = utils.CreateWorkDir()
		KindCfg = WorkDir + "/" + "kind.kubeconfig"
		// cleanup workdir at the end
		defer os.RemoveAll(WorkDir)

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

		CapiCfg := WorkDir + "/" + clusterName + ".kubeconfig"
		gokpartifacts := os.Getenv("HOME") + "/.gokp/" + clusterName

		tcpName := "gokp-bootstrapper"

		// Run PreReq Checks
		_, err := utils.CheckPreReqs(gokpartifacts)
		if err != nil {
			log.Fatal(err)
		}

		// Create KIND instance
		log.Info("Creating temporary control plane")
		err = kind.CreateKindCluster(tcpName, KindCfg)
		if err != nil {
			log.Fatal(err)
		}

		// Create CAPI instance on AWS
		awsCredsMap := map[string]string{
			"AWS_REGION":                     awsRegion,
			"AWS_ACCESS_KEY_ID":              awsAccessKey,
			"AWS_SECRET_ACCESS_KEY":          awsSecretKey,
			"AWS_SSH_KEY_NAME":               awsSSHKey,
			"AWS_CONTROL_PLANE_MACHINE_TYPE": awsCPMachine,
			"AWS_NODE_MACHINE_TYPE":          awsWMachine,
		}

		_, err = capi.CreateAwsK8sInstance(KindCfg, &clusterName, WorkDir, awsCredsMap, CapiCfg)
		if err != nil {
			log.Fatal(err)
		}

		// Create the GitOps repo
		_, gitopsrepo, err := github.CreateRepo(&clusterName, ghToken, &privateRepo, WorkDir)
		if err != nil {
			log.Fatal(err)
		}

		// Create repo dir structure. Including Argo CD install YAMLs and base YAMLs. Push initial dir structure out
		_, err = templates.CreateRepoSkel(&clusterName, WorkDir, ghToken, gitopsrepo, &privateRepo)
		if err != nil {
			log.Fatal(err)
		}

		// Export/Create Cluster YAML to the Repo, Make sure kustomize is used for the core components
		log.Info("Exporting Cluster YAML")
		_, err = export.ExportClusterYaml(CapiCfg, WorkDir+"/"+clusterName)
		if err != nil {
			log.Fatal(err)
		}

		// Git push newly exported YAML to GitOps repo
		_, err = github.CommitAndPush(WorkDir+"/"+clusterName, ghToken, "exporting existing YAML")
		if err != nil {
			log.Fatal(err)
		}

		// Install Argo CD on the newly created cluster
		// Deploy applications/applicationsets
		log.Info("Deploying Argo CD GitOps Controller")
		_, err = argo.BootstrapArgoCD(&clusterName, WorkDir, CapiCfg)
		if err != nil {
			log.Fatal(err)
		}

		// MOVE from kind to capi instance
		//	uses the kubeconfig files of "src ~> dest"
		log.Info("Moving CAPI Artifacts to: " + clusterName)
		_, err = capi.MoveMgmtCluster(KindCfg, CapiCfg)
		if err != nil {
			log.Fatal(err)
		}

		// Delete local Kind Cluster
		log.Info("Deleting temporary control plane")
		err = kind.DeleteKindCluster(tcpName, KindCfg)
		if err != nil {
			log.Fatal(err)
		}

		// Move components to ~/.gokp/<clustername> and remove stuff you don't need to know.
		// Move the directory
		err = os.Rename(WorkDir, gokpartifacts)
		if err != nil {
			log.Fatal(err)
		}

		// Remove files/directories that we may not need
		notNeededDirs := []string{
			"argocd-install-output",
			"capi-install-yamls-output",
			"cni-output",
		}

		for _, notNeededDir := range notNeededDirs {
			err = os.RemoveAll(gokpartifacts + "/" + notNeededDir)
			if err != nil {
				log.Fatal(err)
			}
		}

		notNeededFiles := []string{
			"argocd-install.yaml",
			"cni.yaml",
			"install-cluster.yaml",
			"kind.kubeconfig",
		}

		for _, notNeededFile := range notNeededFiles {
			err = os.Remove(gokpartifacts + "/" + notNeededFile)
			if err != nil {
				log.Fatal(err)
			}
		}

		// Give info
		log.Info("Cluster Successfully installed! Everything you need is under: ~/.gokp/", clusterName)
	},
}

func init() {
	rootCmd.AddCommand(createClusterCmd)
	// Repo specific flags
	createClusterCmd.Flags().String("github-token", "", "GitHub token to use.")
	createClusterCmd.Flags().String("cluster-name", "", "Name of your cluster.")
	createClusterCmd.Flags().BoolP("private-repo", "", true, "Create a private repo.")

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

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// createClusterCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// createClusterCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
