package flux

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/christianh814/gokp/cmd/capi"
	"github.com/christianh814/gokp/cmd/utils"
	"k8s.io/client-go/tools/clientcmd"
)

// BootstrapFluxCD installs FluxCD on a given cluster with the provided Kustomize-ed dir
func BootstrapFluxCD(clustername *string, workdir string, capicfg string) (bool, error) {
	// Set the repoDir path where things should be cloned.
	// check if it exists
	repoDir := workdir + "/" + *clustername
	overlay := repoDir + "/cluster/core/flux-system"
	if _, err := os.Stat(repoDir); os.IsNotExist(err) {
		return false, err
	}

	// generate the FluxCD Install YAML
	fluxcdyaml := workdir + "/" + "flux-install.yaml"
	_, err := utils.RunKustomize(overlay, fluxcdyaml)
	if err != nil {
		return false, err
	}

	// Let's take that YAML and apply it to the created cluster
	// First, let's split this up into smaller files
	err = utils.SplitYamls(workdir+"/"+"fluxcd-install-output", fluxcdyaml, "---")
	if err != nil {
		return false, err
	}

	//get a list of those files
	fluxInstallYamls, err := filepath.Glob(workdir + "/" + "fluxcd-install-output" + "/" + "*.yaml")
	if err != nil {
		return false, err
	}

	// Set up a connection to the K8S cluster and apply these bad boys
	capiInstallConfig, err := clientcmd.BuildConfigFromFlags("", capicfg)
	if err != nil {
		return false, err
	}

	// Loop until all are applied. Set a counter so we don't loop endlessly. Keep track of errors
	counter := 0
	for runs := 15; counter <= runs; counter++ {
		// break if we've tried 15 times (aka 30 seconds)
		if counter > runs {
			return false, errors.New("failed to apply flux manifests")
		}
		// set the error count
		errcount := 0
		// loop through the YAMLS counting the errors
		for _, fluxInstallYaml := range fluxInstallYamls {
			err = capi.DoSSA(context.TODO(), capiInstallConfig, fluxInstallYaml)
			if err != nil {
				errcount++
			}
			// sleep and wait to apply the next one
			time.Sleep(2 * time.Second)
		}
		// If no errors were found, break out of the loop
		if errcount == 0 {
			break
		}
	}

	return true, nil
}
