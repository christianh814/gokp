// maybe this package isn't needed
package export

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var ClusterScopedKustomizeFile string = `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization


commonAnnotations:
    argocd.argoproj.io/sync-options: SkipDryRunOnMissingResource=true
    argocd.argoproj.io/sync-options: Validate=false

resources:
{{- range $ClusterScopedYaml := .ClusterScopedYamls }}
- {{ $ClusterScopedYaml -}}
{{ end }
`

type GroupResource struct {
	APIGroup        string
	APIGroupVersion string
	APIVersion      string
	APIResource     metav1.APIResource
}
