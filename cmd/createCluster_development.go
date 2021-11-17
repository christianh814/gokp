package cmd

import (
	"os"

	"github.com/christianh814/gokp/cmd/argo"
	"github.com/christianh814/gokp/cmd/capi"
	"github.com/christianh814/gokp/cmd/export"
	"github.com/christianh814/gokp/cmd/flux"
	"github.com/christianh814/gokp/cmd/github"
	"github.com/christianh814/gokp/cmd/kind"
	"github.com/christianh814/gokp/cmd/templates"
	"github.com/christianh814/gokp/cmd/utils"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// developmentClusterCmd represents the developmentCluster command
var developmentClusterCmd = &cobra.Command{
	Use:   "development",
	Short: "Creates a local testing cluster using Docker",
	Long: `Create a GitOps Ready K8S Test Cluster using CAPI!

Currenly Docker + GitHub.
	
This is a PoC stage (proof of concept) and should NOT
be used for production. There will be lots of breaking changes
so beware. This create a local cluster for testing. PRE-PRE-ALPHA.`,
	Run: func(cmd *cobra.Command, args []string) {
		// create home dir
		err := os.MkdirAll(os.Getenv("HOME")+"/.gokp", 0775)
		if err != nil {
			log.Fatal(err)
		}
		// Create workdir and set variables based on that
		WorkDir, _ = utils.CreateWorkDir()
		KindCfg = WorkDir + "/" + "kind.kubeconfig"
		// cleanup workdir at the end
		defer os.RemoveAll(WorkDir)

		// Grab repo related flags
		ghToken, _ := cmd.Flags().GetString("github-token")
		clusterName, _ := cmd.Flags().GetString("cluster-name")
		privateRepo, _ := cmd.Flags().GetBool("private-repo")

		// Set GitOps Controller
		gitOpsController, _ := cmd.Flags().GetString("gitops-controller")

		// HA request
		createHaCluster, _ := cmd.Flags().GetBool("ha")

		// Set up cluster artifacts
		CapiCfg := WorkDir + "/" + clusterName + ".kubeconfig"
		gokpartifacts := os.Getenv("HOME") + "/.gokp/" + clusterName

		// set the bootstrapper name
		tcpName := "gokp-bootstrapper"

		// Run PreReq Checks
		_, err = utils.CheckPreReqs(gokpartifacts, gitOpsController)
		if err != nil {
			log.Fatal(err)
		}

		// Create KIND instance
		log.Info("Creating temporary control plane")
		err = kind.CreateCAPDKindCluster(tcpName, KindCfg, WorkDir)
		if err != nil {
			log.Fatal(err)
		}

		// Create Development instance
		_, err = capi.CreateDevelK8sInstance(KindCfg, &clusterName, WorkDir, CapiCfg, createHaCluster)
		if err != nil {
			log.Fatal(err)
		}

		// Create the GitOps repo
		_, gitopsrepo, err := github.CreateRepo(&clusterName, ghToken, &privateRepo, WorkDir)
		if err != nil {
			log.Fatal(err)
		}

		// Create repo dir structure based on which gitops controller that was chosen
		if gitOpsController == "argocd" {
			// Create repo dir structure. Including Argo CD install YAMLs and base YAMLs. Push initial dir structure out
			_, err = templates.CreateArgoRepoSkel(&clusterName, WorkDir, ghToken, gitopsrepo, &privateRepo)
			if err != nil {
				log.Fatal(err)
			}
		} else if gitOpsController == "fluxcd" {
			// Create repo dir structure. Including Flux CD install YAMLs and base YAMLs. Push initial dir structure out
			_, err = templates.CreateFluxRepoSkel(&clusterName, WorkDir, ghToken, gitopsrepo, &privateRepo)
			if err != nil {
				log.Fatal(err)
			}
		} else {
			log.Fatal("unknown gitops controller")
		}

		// Export/Create Cluster YAML to the Repo, Make sure kustomize is used for the core components
		log.Info("Exporting Cluster YAML")
		_, err = export.ExportClusterYaml(CapiCfg, WorkDir+"/"+clusterName, gitOpsController)
		if err != nil {
			log.Fatal(err)
		}

		// Git push newly exported YAML to GitOps repo
		privateKeyFile := WorkDir + "/" + clusterName + "_rsa"
		_, err = github.CommitAndPush(WorkDir+"/"+clusterName, privateKeyFile, "exporting existing YAML")
		if err != nil {
			log.Fatal(err)
		}

		// Deplopy the GitOps controller that was chosen
		if gitOpsController == "argocd" {
			// Install Argo CD on the newly created cluster with applications/applicationsets
			log.Info("Deploying Argo CD GitOps Controller")
			_, err = argo.BootstrapArgoCD(&clusterName, WorkDir, CapiCfg)
			if err != nil {
				log.Fatal(err)
			}
		} else if gitOpsController == "fluxcd" {
			// Install Flux CD on the newly created cluster with all it's components
			log.Info("Deploying Flux CD GitOps Controller")
			_, err = flux.BootstrapFluxCD(&clusterName, WorkDir, CapiCfg)
			if err != nil {
				log.Fatal(err)
			}
		} else {
			log.Fatal("unknown gitops controller")
		}

		// Delete local Kind Cluster
		log.Info("Deleting temporary control plane")
		err = kind.DeleteKindCluster(tcpName, KindCfg)
		if err != nil {
			log.Fatal(err)
		}

		// Move components to ~/.gokp/<clustername> and remove stuff you don't need to know.
		// 	TODO: this is ugly and will refactor this later
		//err = utils.CopyDir(WorkDir, gokpartifacts)
		err = os.Rename(WorkDir, gokpartifacts)
		if err != nil {
			log.Fatal(err)
		}

		notNeeded := []string{
			"argocd-install-output",
			"capi-install-yamls-output",
			"cni-output",
			"fluxcd-install-output",
			"argocd-install.yaml",
			"flux-install.yaml",
			"cni.yaml",
			"install-cluster.yaml",
			"kind.kubeconfig",
			"kindconfig.yaml",
		}

		for _, notNeededthing := range notNeeded {
			err = os.RemoveAll(gokpartifacts + "/" + notNeededthing)
			if err != nil {
				log.Fatal(err)
			}
		}

		// Give info
		log.Info("Cluster Successfully installed! Everything you need is under: ~/.gokp/", clusterName)
	},
}

func init() {
	createClusterCmd.AddCommand(developmentClusterCmd)

	// GitOps Controller Flag
	developmentClusterCmd.Flags().String("gitops-controller", "argocd", "The GitOps Controller to use for this cluster.")

	// Repo Specific Flags
	developmentClusterCmd.Flags().String("github-token", "", "GitHub token to use.")
	developmentClusterCmd.Flags().String("cluster-name", "", "Name of your cluster.")
	developmentClusterCmd.Flags().BoolP("private-repo", "", true, "Create a private repo.")
	developmentClusterCmd.Flags().BoolP("ha", "", false, "Create an HA cluster.")

	// required flags
	developmentClusterCmd.MarkFlagRequired("github-token")
	developmentClusterCmd.MarkFlagRequired("cluster-name")
}
