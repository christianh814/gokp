package capi

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	//"github.com/aws/aws-sdk-go/aws"
	"context"
	"encoding/base64"
	"encoding/json"

	"github.com/aws/aws-sdk-go/aws/session"
	cfn "github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/christianh814/gokp/cmd/utils"
	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/cluster-api-provider-aws/cmd/clusterawsadm/cloudformation/bootstrap"
	cloudformation "sigs.k8s.io/cluster-api-provider-aws/cmd/clusterawsadm/cloudformation/service"
	creds "sigs.k8s.io/cluster-api-provider-aws/cmd/clusterawsadm/credentials"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha4"
	capiclient "sigs.k8s.io/cluster-api/cmd/clusterctl/client"
	kcpv1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1alpha3"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
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

	// TODO: This may not be needed
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
	//		We want to wait for "capa-controller-manager" deployment in the "capa-system" ns to
	//		rollout before we proceed.
	//	TODO: We probably want a generic "watch/rollout" function for things

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
		err = DoSSA(context.TODO(), clusterInstallConfig, yamlFile)
		if err != nil {
			log.Warn("Unable to read YAML: ", err)
			//return false, err
		}
	}

	log.Info("CAPI System Configured")

	// Wait for the controlplane to have 3 nodes and that they are initialized

	//	First, wait for the infra to appear
	_, err = waitForAWSInfra(clusterInstallConfig, *clusterName)
	if err != nil {
		return false, err
	}

	//	Then, wait for the CP to appear
	_, err = waitForCP(clusterInstallConfig, *clusterName)
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
		err = DoSSA(context.TODO(), capiInstallConfig, cniyamlFile)
		if err != nil {
			if !strings.Contains(err.Error(), "is missing in") {
				return false, err
			}
			//log.Warn("Unable to read YAML: ", err)
		}
	}

	log.Info("Successfully installed CNI")

	// Wait until Nodes are READY
	log.Info("Waiting for worker nodes to come online")

	// HACK: We sleep to give time for the CNI to rollout
	//	TODO: Wait until CNI Deployment is done
	time.Sleep(60 * time.Second)

	_, err = waitForReadyNodes(capiInstallConfig)
	if err != nil {
		return false, err
	}

	// Unexport AWS settings
	os.Unsetenv("AWS_B64ENCODED_CREDENTIALS")
	for k := range awscreds {
		os.Unsetenv(k)
	}

	// If we're here, that means everything turned out okay
	log.Info("Successfully created AWS Kubernetes Cluster")
	return true, nil
}

// DoSSA  does service side apply with the given YAML
func DoSSA(ctx context.Context, cfg *rest.Config, yaml string) error {
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

// waitForAWSInfra waits until the infrastructure is provisioned
//	TODO: probably should use https://pkg.go.dev/k8s.io/client-go/tools/watch
func waitForAWSInfra(restConfig *rest.Config, clustername string) (bool, error) {
	// We need to load the scheme since it's not part of the core API
	log.Info("Waiting for AWS Infrastructure")
	scheme := runtime.NewScheme()
	err := clusterv1.AddToScheme(scheme)
	if err != nil {
		return false, err
	}

	c, err := client.New(restConfig, client.Options{
		Scheme: scheme,
	})
	if err != nil {
		return false, err
	}

	// wait up until 40 minutes
	counter := 0
	for runs := 20; counter <= runs; counter++ {
		if counter > runs {
			return false, errors.New("aws infra did not come up after 40 minutes")
		}
		// get the current status, wait for "Provisioned"
		cluster := &clusterv1.Cluster{}
		if err := c.Get(context.TODO(), client.ObjectKey{Namespace: "default", Name: clustername}, cluster); err != nil {
			return false, err
		}
		if cluster.Status.Phase == "Provisioned" {
			break
		}
		time.Sleep(2 * time.Minute)

	}
	return true, nil
}

// waitForCP waits until the CP to come up
//	TODO: probably should use https://pkg.go.dev/k8s.io/client-go/tools/watch
func waitForCP(restConfig *rest.Config, clustername string) (bool, error) {
	log.Info("Waiting for the Control Plane to appear")
	// Set the vars we need
	cpname := clustername + "-control-plane"
	var expectedCPReplicas int32 = 3

	// We need to load the scheme since it's not part of the core API
	scheme := runtime.NewScheme()
	_ = kcpv1.AddToScheme(scheme)

	c, err := client.New(restConfig, client.Options{
		Scheme: scheme,
	})
	if err != nil {
		return false, err
	}

	// wait up until 20 minutes
	counter := 0
	for runs := 20; counter <= runs; counter++ {
		if counter > runs {
			return false, errors.New("control-plane did not come up after 10 minutes")
		}
		// get the current status, wait for 3 CP nodes
		kcp := &kcpv1.KubeadmControlPlane{}
		if err := c.Get(context.TODO(), client.ObjectKey{Namespace: "default", Name: cpname}, kcp); err != nil {
			return false, err
		}
		if kcp.Status.Replicas == expectedCPReplicas {
			break
		}

		time.Sleep(1 * time.Minute)

	}

	return true, nil
}

// waitForReadyNodes waits until all nodes are in a ready state
//	TODO: probably should use https://pkg.go.dev/k8s.io/client-go/tools/watch
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
		// trying to wait until they're ready
		// TODO: Currently doesn't do what I thin kit does. Save for later
		/*
			for _, t := range node.Spec.Taints {
				counter := 0
				for ok := true; ok; ok = (t.Key == "node.kubernetes.io/not-ready") {
					if counter > 50 {
						return false, errors.New("nodes still tainted with not ready")
					}
					time.Sleep(10 * time.Second)
					counter++
				}
			}
		*/
	}
	// if we're here, we're okay
	return true, nil
}

// DeleteCluster deletes the given capi managed cluster
func DeleteCluster(cfg string, name string) (bool, error) {
	// We need to load the scheme since it's not part of the core API
	scheme := runtime.NewScheme()
	err := clusterv1.AddToScheme(scheme)
	if err != nil {
		return false, err
	}

	// Create client and use the scheme
	kindclient, err := clientcmd.BuildConfigFromFlags("", cfg)
	if err != nil {
		return false, err
	}
	c, err := client.New(kindclient, client.Options{
		Scheme: scheme,
	})
	if err != nil {
		return false, err
	}

	// Check if the cluster is there
	cluster := &clusterv1.Cluster{}
	if err := c.Get(context.TODO(), client.ObjectKey{Namespace: "default", Name: name}, cluster); err != nil {
		return false, err
	}

	// Try and delete the cluster
	err = c.Delete(context.TODO(), cluster)
	if err != nil {
		return false, err
	}

	//if we are here we're okay
	return true, nil
}

// MoveMgmtCluster moves the management cluster from src kubeconfig to dest kubeconfig
func MoveMgmtCluster(src string, dest string) (bool, error) {
	// create capi client
	c, err := capiclient.New("")
	if err != nil {
		return false, err
	}

	// Create clientset for src
	srcclient, err := clientcmd.BuildConfigFromFlags("", src)
	if err != nil {
		return false, err
	}
	srcclientset, err := kubernetes.NewForConfig(srcclient)
	if err != nil {
		return false, err
	}

	// Get the secret and base64 encode it (you'd think it would come encoded but it doesn't)
	secret, err := srcclientset.CoreV1().Secrets("capa-system").Get(context.TODO(), "capa-manager-bootstrap-credentials", metav1.GetOptions{})
	if err != nil {
		return false, err
	}

	s := secret.Data["credentials"]
	sb64 := base64.StdEncoding.EncodeToString(s)
	if err != nil {
		return false, err
	}

	// export it into the env
	os.Setenv("AWS_B64ENCODED_CREDENTIALS", sb64)

	// init the dest cluster
	_, err = c.Init(capiclient.InitOptions{
		Kubeconfig:              capiclient.Kubeconfig{Path: dest},
		InfrastructureProviders: []string{"aws"},
	})

	if err != nil {
		return false, err
	}

	// perform the move
	err = c.Move(capiclient.MoveOptions{
		FromKubeconfig: capiclient.Kubeconfig{Path: src},
		ToKubeconfig:   capiclient.Kubeconfig{Path: dest},
	})
	if err != nil {
		return false, err
	}

	// Unset the env var
	os.Unsetenv("AWS_B64ENCODED_CREDENTIALS")

	// if we're here we must be okay
	return true, nil
}
