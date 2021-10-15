package kind

import (
	"github.com/christianh814/gokp/cmd/utils"
	"sigs.k8s.io/kind/pkg/cluster"
)

var CAPDKindConfig string = `kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
  extraMounts:
    - hostPath: /var/run/docker.sock
      containerPath: /var/run/docker.sock
`

// CreateKindCluster creates KIND cluster to use as the temp cluster manager
func CreateKindCluster(name string, cfg string) error {
	/* trying to quiet down KIND*/
	/*
		klogger := kindcmd.NewLogger()
		provider := cluster.NewProvider(
			cluster.ProviderWithLogger(klogger),
		)
	*/

	//create a new KIND provider
	provider := cluster.NewProvider()

	// Create a KIND instance and write out the kubeconfig in the specified location
	err := provider.Create(
		name,
		cluster.CreateWithKubeconfigPath(cfg),
		// setting these to false for now
		cluster.CreateWithDisplayUsage(false),
		cluster.CreateWithDisplaySalutation(false),
	)

	if err != nil {
		return err
	}

	return nil
}

// DeleteKindCluster deletes KIND cluster based on the name given
func DeleteKindCluster(name string, cfg string) error {
	/* Testing quieting down deleting too*/
	/*
		klogger := kindcmd.NewLogger()
		provider := cluster.NewProvider(
			cluster.ProviderWithLogger(klogger),
		)
	*/

	provider := cluster.NewProvider()

	err := provider.Delete(name, cfg)

	if err != nil {
		return err
	}

	return nil

}

// CreateCAPDKindClsuter creates KIND cluster to use as the temp cluster manager for a CAPD deployment
func CreateCAPDKindCluster(name string, cfg string, dir string) error {
	// Writeout the KIND config for CAPD
	kindcfg := dir + "/kindconfig.yaml"
	//dummy vars since we don't need them
	dummyVars := struct {
		Dummykey string
	}{
		Dummykey: "unused",
	}

	// Write out the Kind file based on the vars and the template
	_, err := utils.WriteTemplate(CAPDKindConfig, kindcfg, dummyVars)
	if err != nil {
		return err
	}

	//create a new KIND provider
	provider := cluster.NewProvider()

	// Create a KIND instance and write out the kubeconfig in the specified location
	err = provider.Create(
		name,
		cluster.CreateWithKubeconfigPath(cfg),
		cluster.CreateWithConfigFile(kindcfg),
		cluster.CreateWithDisplayUsage(false),
		cluster.CreateWithDisplaySalutation(false),
	)

	if err != nil {
		return err
	}

	return nil
}

// GetKindKubeconfig returns the Kubeconfig of the named KIND cluster
func GetKindKubeconfig(name string, internal bool) (string, error) {
	// Create a provider and return the named kubeconfig file as a string
	provider := cluster.NewProvider()
	return provider.KubeConfig(name, internal)
}
