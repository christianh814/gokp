package kind

import (
	"sigs.k8s.io/kind/pkg/cluster"
)

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
