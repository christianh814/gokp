package templates

import (
	"os"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/christianh814/project-spichern/cmd/github"
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
{{- if .IsPrivate }}
  username: not-used
  password: {{.GitHubToken}}
{{ end }}
`

var ArgoCdComponetnsApplicationSetKustomize string = `resources:
- cluster-components.yaml
`
var ArgoCdComponentsArgoProjKustomize string = `resources:
- cluster.yaml
`

var ArgoCdClusterComponentApplicationSet string = `
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: cluster
  namespace: argocd
spec:
  generators:
  - git:
      repoURL: {{.ClusterGitOpsRepo}}
      revision: main
      directories:
      - path: cluster/tenants/*
  template:
    metadata:
      name: {{.RawPathBasename}}
    spec:
      project: cluster
      syncPolicy:
        automated:
          prune: true
          selfHeal: true
        retry:
          limit: 15
          backoff:
            duration: 15s
            factor: 2
            maxDuration: 5m
      source:
        repoURL: {{.ClusterGitOpsRepo}}
        targetRevision: main
        path: {{.RawPath}}
      destination:
        server: https://kubernetes.default.svc
`

var ArgoCdComponentsArgoProjProject string = `apiVersion: argoproj.io/v1alpha1
kind: AppProject
metadata:
  name: cluster
  namespace: argocd
spec:
  clusterResourceWhitelist:
  - group: '*'
    kind: '*'
  destinations:
  - namespace: '*'
    server: '*'
  sourceRepos:
  - '*'
`

var ArgoCdTenantArgoKustomize string = `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

commonAnnotations:
    argocd.argoproj.io/sync-options: SkipDryRunOnMissingResource=true
    argocd.argoproj.io/sync-options: Validate=false

bases:
- ../../bootstrap/overlays/default/
`

// CreateRepoSkel creates the skeleton repo structure at the given place
func CreateRepoSkel(name *string, workdir string, ghtoken string, gitopsrepo string, private *bool) (bool, error) {
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
				IsPrivate         bool
			}{
				ClusterGitOpsRepo: gitopsrepo,
				GitHubToken:       ghtoken,
				IsPrivate:         *private,
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

		//	Tenants
		if strings.Contains(dir, "tenants") && strings.Contains(dir, "argocd") {

			// dummy vars for now
			dummyVars := struct {
				Dummykey string
			}{
				Dummykey: "unused",
			}

			// Write out the kustomization file based on the vars and the template
			_, err := utils.WriteTemplate(ArgoCdTenantArgoKustomize, dir+"/"+"kustomization.yaml", dummyVars)
			if err != nil {
				return false, err
			}

		}

	}

	// Commit and push initialize skel
	log.Info("Pushing initial skel repo structure")
	_, err := github.CommitAndPush(repoDir, ghtoken, "initializing skel repo structure")
	if err != nil {
		return false, err
	}
	// If we're here, everything should be okay
	return true, nil
}
