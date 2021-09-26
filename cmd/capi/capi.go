package capi

import (
	"io/ioutil"
	"os"

	//"github.com/aws/aws-sdk-go/aws"
	"context"
	"encoding/json"

	"github.com/aws/aws-sdk-go/aws/session"
	cfn "github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/christianh814/project-spichern/cmd/utils"
	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/cluster-api-provider-aws/cmd/clusterawsadm/cloudformation/bootstrap"
	cloudformation "sigs.k8s.io/cluster-api-provider-aws/cmd/clusterawsadm/cloudformation/service"
	creds "sigs.k8s.io/cluster-api-provider-aws/cmd/clusterawsadm/credentials"
	capiclient "sigs.k8s.io/cluster-api/cmd/clusterctl/client"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
	//capiutil "sigs.k8s.io/cluster-api/util"
)

var CNIurl string = "https://docs.projectcalico.org/v3.20/manifests/calico.yaml"

var decUnstructured = yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)

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
	log.Info("Generating AWS K8S cluster configurations")
	newClient, err := capiclient.New("")
	if err != nil {
		return false, err
	}

	//	Set up options to write out the install YAML
	//	TODO: Make Kubernetes version an option
	var cpMachineCount int64 = 3
	var workerMachineCount int64 = 3
	cto := capiclient.GetClusterTemplateOptions{
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
	//CHX
	log.Info("Configuration complete, installing cluster")
	//	use clientcmd to apply the configuration
	clusterInstallConfig, err := clientcmd.BuildConfigFromFlags("", kindkconfig)
	if err != nil {
		return false, err
	}
	//	Wait for the deployment to rollout
	clusterInstallClientSet, err := kubernetes.NewForConfig(clusterInstallConfig)
	if err != nil {
		return false, err
	}

	dClient := clusterInstallClientSet.AppsV1().Deployments("capa-system")
	depl, err := dClient.Get(context.TODO(), "capa-controller-manager", metav1.GetOptions{})
	if err != nil {
		return false, err
	}

	//	check the status until it's available
	log.Info(depl.Status.AvailableReplicas)
	/*
		for {
			numOfReplicas := depl.Status.AvailableReplicas
			if numOfReplicas == 1 {
				break
			}
		}
	*/
	//	Apply the config now that the capa controller is rolled out
	err = doSSA(context.TODO(), clusterInstallConfig, workdir+"/"+"install-cluster.yaml")

	if err != nil {
		return false, err
	}

	//clusterInstallClientSet.Result

	// Wait for the controlplane to have 3 nodes and that they are initialized

	// Write out CAPI kubeconfig and save it

	//Apply the CNI solution. For now we use Calio
	//	TODO: This should be something that is an end user can choose

	// Wait until all Nodes are READY

	// If we're here, that means everything turned out okay
	log.Info("Successfully created AWS Kubernetes Cluster")
	return true, nil
}

func doSSA(ctx context.Context, cfg *rest.Config, yaml string) error {
	// Read yaml into a slice of byte
	yml, err := ioutil.ReadFile(yaml)
	if err != nil {
		log.Fatal(err)
	}

	// get the RESTMapper for the GVR
	dc, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		return err
	}
	mapper := restmapper.NewDeferredDiscoveryRESTMapper(memory.NewMemCacheClient(dc))

	// create dymanic client
	dyn, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return err
	}

	// read YAML manifest into unstructured.Unstructured
	obj := &unstructured.Unstructured{}
	_, gvk, err := decUnstructured.Decode(yml, nil, obj)
	if err != nil {
		return err
	}

	// Get the GVR
	mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return err
	}

	// Get the REST interface for the GVR
	var dr dynamic.ResourceInterface
	if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
		// namespaced resources should specify the namespace
		dr = dyn.Resource(mapping.Resource).Namespace(obj.GetNamespace())
	} else {
		// for cluster-wide resources
		dr = dyn.Resource(mapping.Resource)
	}

	// Create object into JSON
	data, err := json.Marshal(obj)
	if err != nil {
		return err
	}

	// Create or Update the obj with service side apply
	//     types.ApplyPatchType indicates service side apply
	//     FieldManager specifies the field owner ID.
	_, err = dr.Patch(ctx, obj.GetName(), types.ApplyPatchType, data, metav1.PatchOptions{
		FieldManager: "gokp-bootstrapper",
	})

	return err
}
