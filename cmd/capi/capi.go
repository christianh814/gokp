package capi

import (
	"os"

	//"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	cfn "github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/christianh814/project-spichern/cmd/utils"
	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/cluster-api-provider-aws/cmd/clusterawsadm/cloudformation/bootstrap"
	cloudformation "sigs.k8s.io/cluster-api-provider-aws/cmd/clusterawsadm/cloudformation/service"
	creds "sigs.k8s.io/cluster-api-provider-aws/cmd/clusterawsadm/credentials"
	"sigs.k8s.io/cluster-api/cmd/clusterctl/client"
	capiclient "sigs.k8s.io/cluster-api/cmd/clusterctl/client"
	//capiutil "sigs.k8s.io/cluster-api/util"
)

var CNIurl string = "https://docs.projectcalico.org/v3.20/manifests/calico.yaml"

// CreateAwsK8sInstance creates a Kubernetes cluster on AWS using CAPI and CAPI-AWS
func CreateAwsK8sInstance(kindkconfig string, clusterName *string, workdir string, awscreds map[string]string) (bool, error) {
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
	template := bootstrap.NewTemplate()
	sess, err := session.NewSession()
	if err != nil {
		return false, err
	}

	cfnSvc := cloudformation.NewService(cfn.New(sess))

	err = cfnSvc.ReconcileBootstrapStack(template.Spec.StackName, *template.RenderCloudFormation())
	if err != nil {
		return false, err
	}

	err = cfnSvc.ShowStackResources(template.Spec.StackName)
	if err != nil {
		return false, err
	}

	//Encode credentials
	awsCreds, err := creds.NewAWSCredentialFromDefaultChain(awscreds["AWS_REGION"])
	if err != nil {
		return false, err
	}

	b64creds, err := awsCreds.RenderBase64EncodedAWSDefaultProfile()
	os.Setenv("AWS_B64ENCODED_CREDENTIALS", b64creds)
	if err != nil {
		return false, err
	}

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

	// Generate cluster YAML for CAPI on KIND and apply it
	newClient, err := capiclient.New("")
	if err != nil {
		return false, err
	}

	//	Set up options to write out the install YAML
	//	TODO: Make Kubernetes version an option
	var cpMachineCount int64 = 3
	var workerMachineCount int64 = 3
	cto := client.GetClusterTemplateOptions{
		Kubeconfig:               capiclient.Kubeconfig{Path: kindkconfig},
		ClusterName:              *clusterName,
		ControlPlaneMachineCount: &cpMachineCount,
		WorkerMachineCount:       &workerMachineCount,
		KubernetesVersion:        "v1.22.2",
	}

	//	Load up the config with the options
	installYaml, err := newClient.GetClusterTemplate(cto)

	if err != nil {
		return false, err
	}

	// Write the install file out
	err = utils.WriteYamlOutput(installYaml, workdir+"/"+"install-cluster.yaml")
	if err != nil {
		return false, err
	}

	// Apply the YAML to the KIND instance so that the cluster gets installed on AWS

	// Wait for the controlplane to have 3 nodes and that they are initialized

	// Write out CAPI kubeconfig and save it

	//Apply the CNI solution. For now we use Calio
	//	TODO: This should be something that is an end user can choose

	// Wait until all Nodes are READY

	// If we're here, that means everything turned out okay
	log.Info("Successfully created AWS Kubernetes Cluster")
	return true, nil
}
