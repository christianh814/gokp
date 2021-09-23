package templates

import (
	"os"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/christianh814/project-spichern/cmd/utils"
)

type ArgoCDVersions struct {
	ArgocdVer  string
	AppsetVers string
}

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

// CreateRepoSkel creates the skeleton repo structure at the given place
func CreateRepoSkel(name *string, workdir string) (bool, error) {
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
			argocdinstall := ArgoCDVersions{
				ArgocdVer:  "stable",
				AppsetVers: "v0.2.0",
			}

			// Write out the kustomization file based on the vars and the template
			_, err := utils.WriteTemplate(ArgoKustomizeFile, dir+"/"+"kustomization.yaml", argocdinstall)
			if err != nil {
				return false, err
			}

			// Write out the argocd namespace file based on the vars and the template
			// 	NOTE: No vars needed in this template but we pass them in case we need them later
			_, err = utils.WriteTemplate(ArgoCdNameSpaceFile, dir+"/"+"argocd-ns.yaml", argocdinstall)
			if err != nil {
				return false, err
			}

		}
	}

	// If we're here, everything should be okay
	return true, nil
}
