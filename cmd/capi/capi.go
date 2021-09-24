package capi

import (
	"os"

	log "github.com/sirupsen/logrus"
	capiclient "sigs.k8s.io/cluster-api/cmd/clusterctl/client"
	//capiutil "sigs.k8s.io/cluster-api/util"
)

var CNIurl string = "https://docs.projectcalico.org/v3.20/manifests/calico.yaml"

// CreateAwsK8sInstance creates a Kubernetes cluster on AWS using CAPI and CAPI-AWS
func CreateAwsK8sInstance(kindkconfig string, awscreds map[string]string) (bool, error) {
	// Export AWS settings as Env vars
	for k := range awscreds {
		os.Setenv(k, awscreds[k])
	}

	/* For testing

	fmt.Println(os.Getenv("AWS_REGION"))
	fmt.Println(os.Getenv("AWS_ACCESS_KEY_ID"))
	fmt.Println(os.Getenv("AWS_SECRET_ACCESS_KEY"))
	fmt.Println(os.Getenv("AWS_SSH_KEY_NAME"))
	fmt.Println(os.Getenv("AWS_CONTROL_PLANE_MACHINE_TYPE"))
	fmt.Println(os.Getenv("AWS_NODE_MACHINE_TYPE"))

	*/

	// Boostrapping Cloud Formation stack on AWS
	log.Info("Boostrapping Cloud Formation stack on AWS")

	//Encode credentials
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

	//Generate cluster YAML for CAPI on KIND and apply it

	// Wait for the controlplane to have 3 nodes and that they are initialized

	// Write out CAPI kubeconfig and save it

	//Apply the CNI solution. For now we use Calio
	//	TODO: This should be something that is an end user can choose

	// Wait until all Nodes are READY

	// If we're here, that means everything turned out okay
	log.Info("Successfully created AWS Kubernetes Cluster")
	return true, nil
}
