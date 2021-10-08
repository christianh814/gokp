package kind

import (
	"sigs.k8s.io/kind/pkg/cluster"
	kindcmd "sigs.k8s.io/kind/pkg/cmd"
)

// CreateKindCluster creates KIND cluster to use as the temp cluster manager
func CreateKindCluster(name string, cfg string) error {
	klogger := kindcmd.NewLogger()
	provider := cluster.NewProvider(
		cluster.ProviderWithLogger(klogger),
	)

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

	klogger := kindcmd.NewLogger()
	provider := cluster.NewProvider(
		cluster.ProviderWithLogger(klogger),
	)

	err := provider.Delete(name, cfg)

	if err != nil {
		return err
	}

	return nil

}
