package templates

import (
	"encoding/base64"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/christianh814/gokp/cmd/github"
	"github.com/christianh814/gokp/cmd/utils"
)

// CreateArgoRepoSkel creates the skeleton repo structure at the given place
func CreateArgoRepoSkel(name *string, workdir string, ghtoken string, gitopsrepo string, private *bool) (bool, error) {
	// Repo Dir should be our workdir + the name of our cluster
	repoDir := workdir + "/" + *name
	directories := []string{
		repoDir + "/" + "cluster/bootstrap/base/",
		repoDir + "/" + "cluster/bootstrap/overlays/",
		repoDir + "/" + "cluster/bootstrap/overlays/default",
		repoDir + "/" + "cluster/components/applicationsets/",
		repoDir + "/" + "cluster/components/argocdproj/",
		repoDir + "/" + "cluster/core/argocd/",
		repoDir + "/" + "cluster/tenants/kuard/",
	}

	// check if the dir is there. If not, error out
	if _, err := os.Stat(repoDir); os.IsNotExist(err) {
		return false, err
	}

	// Create directories
	log.Info("Creating skeleton repo structure")
	for _, dir := range directories {
		os.MkdirAll(dir, 0755)

		// Lot's of ifs coming your way
		//	Check to see if I need to install argocd install kustomization
		if strings.Contains(dir, "bootstrap") && strings.Contains(dir, "base") {
			// Set up the vars to go into the template
			argocdinstall := struct {
				ArgocdVer string
			}{
				ArgocdVer: "stable",
			}

			// Write out the kustomization file based on the vars and the template
			_, err := utils.WriteTemplate(ArgoKustomizeFile, dir+"/"+"kustomization.yaml", argocdinstall)
			if err != nil {
				return false, err
			}

			// Write out the argocd namespace file based on the vars and the template
			// 	NOTE: No vars needed in this template but we pass them in because the func needs it
			_, err = utils.WriteTemplate(ArgoCdNameSpaceFile, dir+"/"+"argocd-ns.yaml", argocdinstall)
			if err != nil {
				return false, err
			}

		}

		//	Check to see if I need to install the ArgoCD Overlays
		if strings.Contains(dir, "bootstrap") && strings.Contains(dir, "overlays") && strings.Contains(dir, "default") {
			// setup dummy values because the func needs it
			dummyVars := struct {
				Dummykey string
			}{
				Dummykey: "unused",
			}

			// Write out the kustomization file based on the vars and the template
			_, err := utils.WriteTemplate(ArgoCdOverlayDefaultKustomize, dir+"/"+"kustomization.yaml", dummyVars)
			if err != nil {
				return false, err
			}

			// Write out the argocd configmap based on the vars and template
			_, err = utils.WriteTemplate(ArgoCdOverlayDefaultConfigMap, dir+"/"+"argocd-cm.yaml", dummyVars)
			if err != nil {
				return false, err
			}

			// Write out the argocd secret of the repo based on the vars and template
			sshKeyFile, err := utils.B64EncodeFile(workdir + "/" + *name + "_rsa")
			if err != nil {
				return false, err
			}
			githubInfo := struct {
				ClusterGitOpsRepo string
				SSHPrivateKey     string
				//IsPrivate         bool
			}{
				ClusterGitOpsRepo: base64.StdEncoding.EncodeToString([]byte(gitopsrepo)),
				SSHPrivateKey:     sshKeyFile,
				//GitHubToken:       ghtoken,
				//IsPrivate:         *private,
			}
			_, err = utils.WriteTemplate(ArgoCdOverlayDefaultRepoSecret, dir+"/"+"repo-secret.yaml", githubInfo)
			if err != nil {
				return false, err
			}

		}
		//	Now we move on to the components with appsets
		if strings.Contains(dir, "components") && strings.Contains(dir, "applicationsets") {
			// setup dummy values because the func needs it
			dummyVars := struct {
				Dummykey string
			}{
				Dummykey: "unused",
			}

			// Write out the kustomization file based on the vars and the template
			_, err := utils.WriteTemplate(ArgoCdComponetnsApplicationSetKustomize, dir+"/"+"kustomization.yaml", dummyVars)
			if err != nil {
				return false, err
			}

			// Write out the application set based on the vars and template
			githubInfo := struct {
				ClusterGitOpsRepo string
				RawPathBasename   string
				RawPath           string
			}{
				ClusterGitOpsRepo: gitopsrepo,
				RawPathBasename:   `'{{path.basename}}'`,
				RawPath:           `'{{path}}'`,
			}

			_, err = utils.WriteTemplate(ArgoCdClusterComponentApplicationSet, dir+"/"+"cluster-components.yaml", githubInfo)
			if err != nil {
				return false, err
			}

			_, err = utils.WriteTemplate(ArgoCdTenantApplicationSet, dir+"/"+"tenants.yaml", githubInfo)
			if err != nil {
				return false, err
			}

		}

		//	Components  with argo projects
		if strings.Contains(dir, "components") && strings.Contains(dir, "argocdproj") {

			dummyVars := struct {
				Dummykey string
			}{
				Dummykey: "unused",
			}

			// Write out the kustomization file based on the vars and the template
			_, err := utils.WriteTemplate(ArgoCdComponentsArgoProjKustomize, dir+"/"+"kustomization.yaml", dummyVars)
			if err != nil {
				return false, err
			}

			// Write out the cluster argocd project file based on the vars and the template
			_, err = utils.WriteTemplate(ArgoCdComponentsArgoProjProject, dir+"/"+"cluster.yaml", dummyVars)
			if err != nil {
				return false, err
			}

		}

		//	Core
		if strings.Contains(dir, "core") && strings.Contains(dir, "argocd") {

			// dummy vars for now
			dummyVars := struct {
				Dummykey string
			}{
				Dummykey: "unused",
			}

			// Write out the kustomization file based on the vars and the template
			_, err := utils.WriteTemplate(ArgoCdArgoKustomize, dir+"/"+"kustomization.yaml", dummyVars)
			if err != nil {
				return false, err
			}

		}
		if strings.Contains(dir, "kuard") {

			// dummy vars for now
			dummyVars := struct {
				Dummykey string
			}{
				Dummykey: "unused",
			}

			// Write out the deployment file based on the vars and the template
			_, err := utils.WriteTemplate(KuardSampleAppDeploy, dir+"/"+"kuard-deploy.yaml", dummyVars)
			if err != nil {
				return false, err
			}

			// Write out the deployment file based on the vars and the template
			_, err = utils.WriteTemplate(KuardSampleAppSvc, dir+"/"+"kuard-service.yaml", dummyVars)
			if err != nil {
				return false, err
			}

			// Write out the deployment file based on the vars and the template
			_, err = utils.WriteTemplate(KuardSampleAppNS, dir+"/"+"kuard-ns.yaml", dummyVars)
			if err != nil {
				return false, err
			}

		}

	}

	// Commit and push initialize skel
	log.Info("Pushing initial skel repo structure")
	privateKeyFile := workdir + "/" + *name + "_rsa"
	_, err := github.CommitAndPush(repoDir, privateKeyFile, "initializing skel repo structure")
	if err != nil {
		return false, err
	}
	// If we're here, everything should be okay
	return true, nil
}

// CreateFluxRepoSkel creates the skeleton repo structure at the given place
func CreateFluxRepoSkel(name *string, workdir string, ghtoken string, gitopsrepo string, private *bool) (bool, error) {
	// Repo Dir should be our workdir + the name of our cluster
	repoDir := workdir + "/" + *name
	directories := []string{
		repoDir + "/" + "cluster/core/flux-system/",
		repoDir + "/" + "cluster/core/cluster-extras/",
		repoDir + "/" + "cluster/tenants/kuard/",
	}

	// check if the dir is there. If not, error out
	if _, err := os.Stat(repoDir); os.IsNotExist(err) {
		return false, err
	}

	// Create directories
	log.Info("Creating skeleton repo structure")
	for _, dir := range directories {
		os.MkdirAll(dir, 0755)

		// Lot's of ifs coming your way

		//	flux-system
		if strings.Contains(dir, "core") && strings.Contains(dir, "flux-system") {

			// Set the version of Flux we want to install
			FluxInstallVars := struct {
				FluxcdVersion string
			}{
				FluxcdVersion: "v0.23.0",
			}

			// Write out the flux-system kustomization file based on the vars and the template
			_, err := utils.WriteTemplate(FluxKustomizeFile, dir+"/"+"kustomization.yaml", FluxInstallVars)
			if err != nil {
				return false, err
			}

			// Set the Vars for the git ssh secret
			privateKeyB64, _ := utils.B64EncodeFile(workdir + "/" + *name + "_rsa")
			publicKeyB64, _ := utils.B64EncodeFile(workdir + "/" + *name + "_rsa.pub")
			SshSecretVars := struct {
				ClusterGitPrivateKey string
				ClusterGitPublicKey  string
			}{
				ClusterGitPrivateKey: privateKeyB64,
				ClusterGitPublicKey:  publicKeyB64,
			}

			// Write out the GitRepository file based on the vars and the template
			_, err = utils.WriteTemplate(FluxGitSshSecret, dir+"/"+"cluster-sshsecret.yaml", SshSecretVars)
			if err != nil {
				return false, err
			}

			// Set the GitRepoURI
			GitRepoURIVars := struct {
				GitRepoURI string
			}{
				GitRepoURI: "ssh://" + strings.ReplaceAll(gitopsrepo, ":", "/"),
			}

			// Write out the GitRepository file based on the vars and the template
			_, err = utils.WriteTemplate(FluxGotkGitRepoFile, dir+"/"+"cluster-gitrepo.yaml", GitRepoURIVars)
			if err != nil {
				return false, err
			}

			// dummy vars for now
			dummyVars := struct {
				Dummykey string
			}{
				Dummykey: "unused",
			}

			// Write out the Kustomization file. No vars needed so we dummy them
			_, err = utils.WriteTemplate(FluxGotkKustomizationFile, dir+"/"+"cluster-kustomization.yaml", dummyVars)
			if err != nil {
				return false, err
			}

			// Write out the flux-system install YAML
			_, err = utils.WriteTemplate(FluxInstallFile, dir+"/"+"flux-system.yaml", dummyVars)
			if err != nil {
				return false, err
			}

		}
		//	cluster-extras
		if strings.Contains(dir, "core") && strings.Contains(dir, "cluster-extras") {

			// Set the version of Flux we want to install
			FluxInstallVars := struct {
				FluxcdVersion string
			}{
				FluxcdVersion: "v0.23.0",
			}

			// Write out the flux-system kustomization file based on the vars and the template
			_, err := utils.WriteTemplate(FluxGotkTenantsFile, dir+"/"+"cluster-tenants.yaml", FluxInstallVars)
			if err != nil {
				return false, err
			}

		}

		//	Sample workload based on kuard
		if strings.Contains(dir, "kuard") {

			// dummy vars for now
			dummyVars := struct {
				Dummykey string
			}{
				Dummykey: "unused",
			}

			// Write out the deployment file based on the vars and the template
			_, err := utils.WriteTemplate(KuardSampleAppDeploy, dir+"/"+"kuard-deploy.yaml", dummyVars)
			if err != nil {
				return false, err
			}

			// Write out the deployment file based on the vars and the template
			_, err = utils.WriteTemplate(KuardSampleAppSvc, dir+"/"+"kuard-service.yaml", dummyVars)
			if err != nil {
				return false, err
			}

			// Write out the deployment file based on the vars and the template
			_, err = utils.WriteTemplate(KuardSampleAppNS, dir+"/"+"kuard-ns.yaml", dummyVars)
			if err != nil {
				return false, err
			}

		}

	}

	// Commit and push initialize skel
	log.Info("Pushing initial skel repo structure")
	privateKeyFile := workdir + "/" + *name + "_rsa"
	_, err := github.CommitAndPush(repoDir, privateKeyFile, "initializing skel repo structure")
	if err != nil {
		return false, err
	}
	// If we're here, everything should be okay
	return true, nil
}
