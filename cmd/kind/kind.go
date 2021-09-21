package kind

import (
	"sigs.k8s.io/kind/pkg/cluster"
	kindcmd "sigs.k8s.io/kind/pkg/cmd"
)

func CreateKindCluster(name string, cfg string) error {
	klogger := kindcmd.NewLogger()
	provider := cluster.NewProvider(
		cluster.ProviderWithLogger(klogger),
	)

	err := provider.Create(
		name,
		cluster.CreateWithKubeconfigPath(cfg),
		cluster.CreateWithDisplayUsage(true),
		cluster.CreateWithDisplaySalutation(true),
	)

	if err != nil {
		return err
	}

	return nil
}
