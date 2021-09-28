package capi

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"time"

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
func CreateAwsK8sInstance(kindkconfig string, clusterName *string, workdir string, awscreds map[string]string, capicfg string) (bool, error) {
	// Export AWS settings as Env vars
	for k := range awscreds {
		os.Setenv(k, awscreds[k])
	}

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
		TargetNamespace:          "default",
	}

	//	Load up the config with the options
	installYaml, err := newClient.GetClusterTemplate(cto)

	if err != nil {
		return false, err
	}

	// Write the install file out
	installClusterYaml := workdir + "/" + "install-cluster.yaml"
	err = utils.WriteYamlOutput(installYaml, installClusterYaml)
	if err != nil {
		return false, err
	}

	// Apply the YAML to the KIND instance so that the cluster gets installed on AWS
	log.Info("Configuration complete, installing cluster")

	//	use clientcmd to apply the configuration
	clusterInstallConfig, err := clientcmd.BuildConfigFromFlags("", kindkconfig)
	if err != nil {
		return false, err
	}

	//	Wait for the deployment to rollout
	//		TODO: There's probably a better way of doing this.
	//		We want to wait for "capa-controller-manager" deployment in the "capa-system" ns to
	//		rollout before we proceed. Sleeping for now
	//time.Sleep(15 * time.Second)

	// Create clientset to check the status
	clientset, err := kubernetes.NewForConfig(clusterInstallConfig)
	if err != nil {
		return false, err
	}

	// Check to see if it's rolled out, if not then wait 5 seconds and check again. Stop after 10x
	counter := 0
	for runs := 10; counter <= runs; counter++ {
		capaClient := clientset.AppsV1().Deployments("capa-system")
		if counter > runs {
			return false, errors.New("CAPI Controller took too long to roll out")
		}
		capaDeployment, err := capaClient.Get(context.TODO(), "capa-controller-manager", metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		availableReplicas := capaDeployment.Status.AvailableReplicas
		if availableReplicas > int32(0) {
			time.Sleep(5 * time.Second)
			break
		}
		time.Sleep(5 * time.Second)
	}
	log.Info("CAPI System Online")

	//	Apply the config now that the capa controller is rolled out

	//	Split the one yaml CAPI gives you into individual files
	err = utils.SplitYamls(workdir+"/"+"capi-install-yamls-output", installClusterYaml, "---")
	if err != nil {
		return false, err
	}

	//	get a list of those files
	yamlFiles, err := filepath.Glob(workdir + "/" + "capi-install-yamls-output" + "/" + "*.yaml")
	if err != nil {
		return false, err
	}

	for _, yamlFile := range yamlFiles {
		err = doSSA(context.TODO(), clusterInstallConfig, yamlFile)
		if err != nil {
			return false, err
		}
	}

	log.Info("CAPI System Configured")

	// Wait for the controlplane to have 3 nodes and that they are initialized
	//	TODO: this function needs to be converted into client-go
	_, err = waitForControlPlane(kindkconfig, *clusterName)
	if err != nil {
		return false, err
	}
	log.Info("Control Plane Nodes are Online, saving Kubeconfig")

	// Write out CAPI kubeconfig and save it
	clusterKubeconfig, err := c.GetKubeconfig(capiclient.GetKubeconfigOptions{
		Kubeconfig:          capiclient.Kubeconfig{Path: kindkconfig},
		WorkloadClusterName: *clusterName,
	})
	if err != nil {
		return false, err
	}

	clusterkcfg, err := os.Create(capicfg)
	if err != nil {
		return false, err
	}
	clusterkcfg.WriteString(clusterKubeconfig)
	clusterkcfg.Close()

	//Apply the CNI solution. For now we use Calico
	//	TODO: This should be something that is an end user can choose

	// Set up the Capi CFG connection
	capiInstallConfig, err := clientcmd.BuildConfigFromFlags("", capicfg)
	if err != nil {
		return false, err
	}
	//	Download the CNI YAML
	cniYaml := workdir + "/" + "cni.yaml"
	_, err = utils.DownloadFile(cniYaml, CNIurl)
	if err != nil {
		return false, err
	}

	//	Split the  CNI yaml into individual files
	err = utils.SplitYamls(workdir+"/"+"cni-output", cniYaml, "---")
	if err != nil {
		return false, err
	}

	//	get a list of those files
	cniyamlFiles, err := filepath.Glob(workdir + "/" + "cni-output" + "/" + "*.yaml")
	if err != nil {
		return false, err
	}

	for _, cniyamlFile := range cniyamlFiles {
		err = doSSA(context.TODO(), capiInstallConfig, cniyamlFile)
		if err != nil {
			return false, err
		}
	}
	log.Info("Successfully installed CNI")

	// Wait until Nodes are READY
	//CHX
	log.Info("Waiting for worker nodes to come online")
	_, err = waitForReadyNodes(capiInstallConfig)
	if err != nil {
		return false, err
	}

	// Unexport AWS settings
	for k := range awscreds {
		os.Unsetenv(k)
	}

	// If we're here, that means everything turned out okay
	log.Info("Successfully created AWS Kubernetes Cluster")
	return true, nil
}

// doSSA  does service side apply with the given YAML
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

// waitForControlPlane waits until the CP returns 3 replicas
//	TODO: turn this into a capi utils call or use client-go
func waitForControlPlane(kubeconfig string, clustername string) (bool, error) {
	cp := 3
	c := 0
	for r := 6; c <= r; c++ {
		if c == r {
			return false, errors.New("controlplane took too long")
		}
		out, err := exec.Command("kubectl", "--kubeconfig", kubeconfig, "kubeadmcontrolplane", clustername+"-control-plane", "-o", "jsonpath='{.status.replicas}'").Output()
		if err != nil {
			log.Info(out)
			return false, err
		}
		if string(out) == fmt.Sprint(cp) {
			break
		}
		log.Info("Current number of CP: ", out)
		time.Sleep(30 * time.Second)
	}
	return true, nil
}

// waitForReadyNodes waits until all nodes are in a ready state
func waitForReadyNodes(cfg *rest.Config) (bool, error) {
	nodesClientSet, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return false, err
	}

	// Get All nodes
	nodesClient, err := nodesClientSet.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return false, err
	}

	// Let's range over all nodes and wait until they're ready
	for _, node := range nodesClient.Items {
		// We are going to range over the conditions
		for _, i := range node.Status.Conditions {
			// we only care about if the Kublet is ready
			if i.Reason == "KubeletReady" {
				// set a counter so we don't run forever
				c := 0
				for r := 100; c <= r; c++ {
					// If we're here, we waited too long
					if c > r {
						return false, errors.New("nodes took too long to come up")
					}
					if i.Type == "Ready" {
						break
					}
					//sleep for 10 seconds
					time.Sleep(10 * time.Second)
				}

			}
		}
	}
	// if we're here, we're okay
	return true, nil
}
