package capi

import (
	log "github.com/sirupsen/logrus"
	capiclient "sigs.k8s.io/cluster-api/cmd/clusterctl/client"
	//capiutil "sigs.k8s.io/cluster-api/util"
)

// CreateAwsK8sInstance creates a Kubernetes cluster on AWS using CAPI and CAPI-AWS
func CreateAwsK8sInstance(kindkconfig string) (bool, error) {
	// Boostrapping Cloud Formation stack on AWS
	log.Info("Boostrapping Cloud Formation stack on AWS")

	// init AWS provider into the Kind instance
	log.Info("Initializing AWS provider into the Kind instance")

	c, err := capiclient.New("")
	if err != nil {
		return false, err
	}

	_, err = c.Init(capiclient.InitOptions{
		Kubeconfig:              capiclient.Kubeconfig{Path: kindkconfig},
		InfrastructureProviders: []string{"aws"},
	})

	if err != nil {
		return false, err
	}
	// If we're here, that means everything turned out okay
	log.Info("Successfully created AWS Kubernetes Cluster")
	return true, nil
}
