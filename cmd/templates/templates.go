package templates

import (
	"os"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/christianh814/project-spichern/cmd/utils"
)

var ArgoKustomizeFile string = `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
namespace: argocd

resources:
- argocd-ns.yaml
- https://raw.githubusercontent.com/argoproj/argo-cd/{{.ArgocdVer}}/manifests/install.yaml
- https://raw.githubusercontent.com/argoproj-labs/applicationset/{{.AppsetVers}}/manifests/install.yaml
`

var ArgoCdNameSpaceFile string = `apiVersion: v1
kind: Namespace
metadata:
  name: argocd
spec: {}
status: {}
`

var ArgoCdOverlayDefaultKustomize string = `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

patchesStrategicMerge:
- argocd-cm.yaml
resources:
- repo-secret.yaml
bases:
- ../../base
- ../../../components/argocdproj
- ../../../components/applicationsets
`

var ArgoCdOverlayDefaultConfigMap string = `apiVersion: v1
kind: ConfigMap
metadata:
  labels:
    app.kubernetes.io/name: argocd-cm
    app.kubernetes.io/part-of: argocd
  name: argocd-cm
  namespace: argocd
data:
  resource.customizations: |
    storage.k8s.io/CSINode:
      ignoreDifferences: |
        jsonPointers:
        - /spec/drivers
    crd.projectcalico.org/IPAMBlock:
      ignoreDifferences: |
        jsonPointers:
        - /spec/allocations
`

var ArgoCdOverlayDefaultRepoSecret string = `apiVersion: v1
kind: Secret
metadata:
  name: cluster-repo
  namespace: argocd
  labels:
    argocd.argoproj.io/secret-type: repository
stringData:
  url: {{.ClusterGitOpsRepo}}
  password: {{.GitHubToken}}
  username: not-used
`

var ArgoCdComponetnsApplicationSetKustomize string = `resources:
- cluster-components.yaml
`

// CreateRepoSkel creates the skeleton repo structure at the given place
func CreateRepoSkel(name *string, workdir string, ghtoken string, gitopsrepo string) (bool, error) {
	// Repo Dir should be our workdir + the name of our cluster
	repoDir := workdir + "/" + *name
	directories := []string{
		repoDir + "/" + "cluster/bootstrap/base/",
		repoDir + "/" + "cluster/bootstrap/overlays/",
		repoDir + "/" + "cluster/bootstrap/overlays/default",
		repoDir + "/" + "cluster/components/applicationsets/",
		repoDir + "/" + "cluster/components/argocdproj/",
		repoDir + "/" + "cluster/tenants/argocd/",
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
				ArgocdVer  string
				AppsetVers string
			}{
				ArgocdVer:  "stable",
				AppsetVers: "v0.2.0",
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
			githubInfo := struct {
				ClusterGitOpsRepo string
				GitHubToken       string
			}{
				ClusterGitOpsRepo: gitopsrepo,
				GitHubToken:       ghtoken,
			}
			_, err = utils.WriteTemplate(ArgoCdOverlayDefaultRepoSecret, dir+"/"+"repo-secret.yaml", githubInfo)
			if err != nil {
				return false, err
			}

		}
		//	Now we move on to the components
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

		}

	}

	// If we're here, everything should be okay
	return true, nil
}
