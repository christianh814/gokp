package capi

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	osruntime "runtime"
	"strings"
	"time"

	"context"
	"encoding/base64"
	"encoding/json"

	"github.com/aws/aws-sdk-go/aws/session"
	cfn "github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/christianh814/gokp/cmd/kind"
	"github.com/christianh814/gokp/cmd/utils"
	"github.com/rwtodd/Go.Sed/sed"
	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/cluster-api-provider-aws/cmd/clusterawsadm/cloudformation/bootstrap"
	cloudformation "sigs.k8s.io/cluster-api-provider-aws/cmd/clusterawsadm/cloudformation/service"
	creds "sigs.k8s.io/cluster-api-provider-aws/cmd/clusterawsadm/credentials"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	capiclient "sigs.k8s.io/cluster-api/cmd/clusterctl/client"
	kcpv1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"

	corev1 "k8s.io/api/core/v1"
	coreV1Types "k8s.io/client-go/kubernetes/typed/core/v1"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
)

var CNIurl string = "https://docs.projectcalico.org/v3.20/manifests/calico.yaml"
var azureCNIurl string = "https://raw.githubusercontent.com/kubernetes-sigs/cluster-api-provider-azure/main/templates/addons/calico.yaml"
var decUnstructured = yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)

var KubernetesVersion string = "v1.22.2"

func CreateAzureK8sInstance(kindkconfig string, clusterName *string, workdir string, azureCredsMap map[string]string, capicfg string, createHaCluster bool) (bool, error) {
	log.Info("Started creating Azure cluster")
	log.Info(kindkconfig)

	var secretsClient coreV1Types.SecretInterface

	// Set up variables
	var cpMachineCount int64
	var workerMachineCount int64
	log.Info("Setting up credentials.")

	for k := range azureCredsMap {
		os.Setenv(k, azureCredsMap[k])
	}
	os.Setenv("AZURE_CLUSTER_IDENTITY_SECRET_NAME", "cluster-identity-secret")
	os.Setenv("AZURE_CLUSTER_IDENTITY_SECRET_NAMESPACE", "default")
	os.Setenv("CLUSTER_IDENTITY_NAME", "cluster-identity")
	clusterInstallConfig, err := clientcmd.BuildConfigFromFlags("", kindkconfig)
	//log.Info(clusterInstallConfig)
	if err != nil {
		return false, err
	}

	clientset, err := kubernetes.NewForConfig(clusterInstallConfig)
	if err != nil {
		return false, err
	}

	secretsClient = clientset.CoreV1().Secrets("default")

	spClientSecret := os.Getenv("AZURE_CLIENT_SECRET")
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cluster-identity-secret",
			Namespace: "default",
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{"clientSecret": []byte(spClientSecret)},
	}

	_, err = secretsClient.Create(context.TODO(), secret, metav1.CreateOptions{})
	if err != nil {
		return false, err
	}
	log.Info("created sp secret")

	// init Azure provider into the Kind instance
	log.Info("Initializing Azure provider")
	c, err := capiclient.New("")
	if err != nil {
		return false, err
	}

	_, err = c.Init(capiclient.InitOptions{
		Kubeconfig:              capiclient.Kubeconfig{Path: kindkconfig},
		InfrastructureProviders: []string{"azure"},
		LogUsageInstructions:    false,
	})

	if err != nil {
		return false, err
	}

	// Check to see if it's rolled out, if not then wait 20 seconds and check again. Stop after 15x
	counter := 0
	for runs := 15; counter <= runs; counter++ {
		capaClient := clientset.AppsV1().Deployments("capz-system")
		if counter > runs {
			return false, errors.New("CAPI Controller took too long to roll out")
		}
		capaDeployment, err := capaClient.Get(context.TODO(), "capz-controller-manager", metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		availableReplicas := capaDeployment.Status.AvailableReplicas
		if availableReplicas > int32(0) {
			time.Sleep(20 * time.Second)
			break
		}
		time.Sleep(20 * time.Second)
	}
	log.Info("creating azureidentity")
	dynamic := dynamic.NewForConfigOrDie(clusterInstallConfig)

	identity := &infrav1.AzureClusterIdentity{
		TypeMeta: metav1.TypeMeta{
			Kind:       "AzureClusterIdentity",
			APIVersion: "infrastructure.cluster.x-k8s.io/v1beta1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "cluster-identity",
		},
		Spec: infrav1.AzureClusterIdentitySpec{
			Type:         infrav1.ServicePrincipal,
			ClientID:     os.Getenv("AZURE_CLIENT_ID"),
			ClientSecret: corev1.SecretReference{Name: "cluster-identity-secret"},
			TenantID:     os.Getenv("AZURE_TENANT_ID"),
			AllowedNamespaces: &infrav1.AllowedNamespaces{
				NamespaceList: []string{"default"},
			},
		},
	}

	resourceId := schema.GroupVersionResource{
		Group:    "infrastructure.cluster.x-k8s.io",
		Version:  "v1beta1",
		Resource: "azureclusteridentities",
	}
	//identity_json, err := json.Marshal(identity)
	if err != nil {
		return false, err
	}
	identity_temp, err := runtime.DefaultUnstructuredConverter.ToUnstructured(identity)
	if err != nil {
		return false, err
	}
	identity_uns := &unstructured.Unstructured{
		Object: identity_temp,
	}

	log.Info("trying to create azureidentity")
	_, err = dynamic.Resource(resourceId).Namespace("default").Create(context.TODO(), identity_uns, metav1.CreateOptions{})
	if err != nil {
		return false, err
	}
	os.Setenv("AZURE_RESOURCE_GROUP", "capz-"+*clusterName)

	// Generate cluster YAML for CAPI on KIND and apply it
	newClient, err := capiclient.New("")
	if err != nil {
		return false, err
	}

	//	Set up options to write out the install YAML
	//	TODO: Make Kubernetes version an option
	if createHaCluster {
		// If HA was requested we create it
		cpMachineCount = 3
		workerMachineCount = 3
		//
	} else {
		// If HA was NOT requested we create a small cluster
		cpMachineCount = 1
		workerMachineCount = 2
	}
	cto := capiclient.GetClusterTemplateOptions{
		Kubeconfig:               capiclient.Kubeconfig{Path: kindkconfig},
		ClusterName:              *clusterName,
		ControlPlaneMachineCount: &cpMachineCount,
		WorkerMachineCount:       &workerMachineCount,
		KubernetesVersion:        KubernetesVersion,
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
	log.Info("Preflight complete, installing cluster")
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
	//	use clientcmd to apply the configuration

	log.Info("submitted cluster config")

	//	First, wait for the infra to appear
	_, err = waitForAWSInfra(clusterInstallConfig, *clusterName)
	if err != nil {
		return false, err
	}

	//	Then, wait for the CP to appear
	_, err = waitForCP(clusterInstallConfig, *clusterName, createHaCluster)
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
	_, err = utils.DownloadFile(cniYaml, azureCNIurl)
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

	// Wait until Nodes are READY
	log.Info("Waiting for worker nodes to come online")

	// HACK: We sleep to give time for the CNI to rollout
	//	TODO: Wait until CNI Deployment is done
	time.Sleep(time.Minute)

	_, err = waitForReadyNodes(capiInstallConfig)
	if err != nil {
		return false, err
	}

	// Unexport AWS settings
	os.Unsetenv("AWS_B64ENCODED_CREDENTIALS")
	for k := range azureCredsMap {
		os.Unsetenv(k)
	}

	// If we're here, that means everything turned out okay
	log.Info("Successfully created Azure Kubernetes Cluster")
	return true, nil
}

// CreateAwsK8sInstance creates a Kubernetes cluster on AWS using CAPI and CAPI-AWS
func CreateAwsK8sInstance(kindkconfig string, clusterName *string, workdir string, awscreds map[string]string, capicfg string, createHaCluster bool, skipCloudFormation bool) (bool, error) {
	// Export AWS settings as Env vars
	for k := range awscreds {
		os.Setenv(k, awscreds[k])
	}

	// Set up variables
	var cpMachineCount int64
	var workerMachineCount int64

	// Boostrapping Cloud Formation stack on AWS only if needed
	if !skipCloudFormation {

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

	} else {
		log.Info("Skipping CloudFormation Creation")
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
	log.Info("Initializing AWS provider")

	c, err := capiclient.New("")
	if err != nil {
		return false, err
	}

	_, err = c.Init(capiclient.InitOptions{
		Kubeconfig:              capiclient.Kubeconfig{Path: kindkconfig},
		InfrastructureProviders: []string{"aws"},
		LogUsageInstructions:    false,
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
	if createHaCluster {
		// If HA was requested we create it
		cpMachineCount = 3
		workerMachineCount = 3
		//
	} else {
		// If HA was NOT requested we create a small cluster
		cpMachineCount = 1
		workerMachineCount = 2
	}
	cto := capiclient.GetClusterTemplateOptions{
		Kubeconfig:               capiclient.Kubeconfig{Path: kindkconfig},
		ClusterName:              *clusterName,
		ControlPlaneMachineCount: &cpMachineCount,
		WorkerMachineCount:       &workerMachineCount,
		KubernetesVersion:        KubernetesVersion,
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
	log.Info("Preflight complete, installing cluster")

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

	// Wait for the controlplane to have 3 nodes and that they are initialized

	//	First, wait for the infra to appear
	_, err = waitForAWSInfra(clusterInstallConfig, *clusterName)
	if err != nil {
		return false, err
	}

	//	Then, wait for the CP to appear
	_, err = waitForCP(clusterInstallConfig, *clusterName, createHaCluster)
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

	// Wait until Nodes are READY
	log.Info("Waiting for worker nodes to come online")

	// HACK: We sleep to give time for the CNI to rollout
	//	TODO: Wait until CNI Deployment is done
	time.Sleep(time.Minute)

	_, err = waitForReadyNodes(capiInstallConfig)
	if err != nil {
		return false, err
	}

	// Unexport AWS settings
	for k := range awscreds {
		os.Unsetenv(k)
	}

	// If we're here, that means everything turned out okay
	log.Info("Successfully created Azure Kubernetes Cluster")
	return true, nil
}

// CreateDevelK8sInstance creates a K8S cluster on Docker
func CreateDevelK8sInstance(kindkconfig string, clusterName *string, workdir string, capicfg string, createHaCluster bool) (bool, error) {
	log.Info("Initializing Docker provider")
	var cpMachineCount int64
	var workerMachineCount int64

	c, err := capiclient.New("")
	if err != nil {
		return false, err
	}

	_, err = c.Init(capiclient.InitOptions{
		Kubeconfig:              capiclient.Kubeconfig{Path: kindkconfig},
		InfrastructureProviders: []string{"docker"},
		LogUsageInstructions:    false,
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
	if createHaCluster {
		// If HA was requested we create it
		cpMachineCount = 3
		workerMachineCount = 3
		//
	} else {
		// If HA was NOT requested we create a small cluster
		cpMachineCount = 1
		workerMachineCount = 2
	}
	cto := capiclient.GetClusterTemplateOptions{
		Kubeconfig:               capiclient.Kubeconfig{Path: kindkconfig},
		ClusterName:              *clusterName,
		ControlPlaneMachineCount: &cpMachineCount,
		WorkerMachineCount:       &workerMachineCount,
		KubernetesVersion:        KubernetesVersion,
		TargetNamespace:          "default",
		ProviderRepositorySource: &capiclient.ProviderRepositorySourceOptions{Flavor: "development"},
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
	log.Info("Preflight complete, installing cluster")

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
		capaClient := clientset.AppsV1().Deployments("capd-system")
		if counter > runs {
			return false, errors.New("CAPI Controller took too long to roll out")
		}
		capdDeployment, err := capaClient.Get(context.TODO(), "capd-controller-manager", metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		availableReplicas := capdDeployment.Status.AvailableReplicas
		if availableReplicas > int32(0) {
			time.Sleep(5 * time.Second)
			break
		}
		time.Sleep(5 * time.Second)
	}

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

	// Wait for the controlplane to have 3 nodes and that they are initialized

	//	First, wait for the infra to appear. This function is badly named
	//	but it should still work even for capd
	_, err = waitForAWSInfra(clusterInstallConfig, *clusterName)
	if err != nil {
		return false, err
	}

	//	Then, wait for the CP to appear
	_, err = waitForCP(clusterInstallConfig, *clusterName, createHaCluster)
	if err != nil {
		return false, err
	}

	log.Info("Control Plane Nodes are Online, saving Kubeconfig")

	// Write out CAPI kubeconfig and save it
	var clusterKubeconfig string
	if osruntime.GOOS == "darwin" {
		// HACK: If we are on a mac we have to modify the file first
		dirtyKK, err := kind.GetKindKubeconfig(*clusterName, false)
		if err != nil {
			return false, err
		}
		// Let's try this sed thing
		engine, err := sed.New(strings.NewReader(`s/0.0.0.0/127.0.0.1/g s/certificate-authority-data:.*/insecure-skip-tls-verify: true/g`))
		if err != nil {
			return false, err
		}
		// set clusterKubeconfig
		clusterKubeconfig, err = engine.RunString(dirtyKK)
		if err != nil {
			return false, err
		}
	} else {
		// If we are on Linux we'll get it the "regular" way
		clusterKubeconfig, err = c.GetKubeconfig(capiclient.GetKubeconfigOptions{
			Kubeconfig:          capiclient.Kubeconfig{Path: kindkconfig},
			WorkloadClusterName: *clusterName,
		})
		if err != nil {
			return false, err
		}

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

	// Wait until Nodes are READY
	log.Info("Waiting for worker nodes to come online")

	// HACK: We sleep to give time for the CNI to rollout
	//	TODO: Wait until CNI Deployment is done
	time.Sleep(time.Minute)

	_, err = waitForReadyNodes(capiInstallConfig)
	if err != nil {
		return false, err
	}

	// if we're here we must be okay
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
	log.Info("Waiting for Infrastructure")
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
		time.Sleep(time.Minute)

	}
	return true, nil
}

// waitForCP waits until the CP to come up
//	TODO: probably should use https://pkg.go.dev/k8s.io/client-go/tools/watch
func waitForCP(restConfig *rest.Config, clustername string, createHaCluster bool) (bool, error) {
	log.Info("Waiting for the Control Plane to appear")
	// Set the vars we need
	cpname := clustername + "-control-plane"
	var expectedCPReplicas int32

	if createHaCluster {
		expectedCPReplicas = 3
	} else {
		expectedCPReplicas = 1

	}

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

		time.Sleep(time.Minute)

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
	// Label workers as such - First select the non control-plane nodes
	workers, err := nodesClientSet.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{
		LabelSelector: `!node-role.kubernetes.io/control-plane`,
	})
	if err != nil {
		return false, err
	}
	// Loop through and label these nodes as workers
	for _, w := range workers.Items {
		// set up the key and value for the worker
		labelKey := "node-role.kubernetes.io/worker"
		labelValue := ""

		// Apply the labels on the Node object
		labels := w.Labels
		labels[labelKey] = labelValue
		w.SetLabels(labels)

		// Tell the API to update the node
		nodesClientSet.CoreV1().Nodes().Update(context.TODO(), &w, metav1.UpdateOptions{})
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

	// Make sure the cluster is ready to be deleted
	_, err = waitForAWSInfra(kindclient, name)
	if err != nil {
		return false, err
	}

	// Try and delete the cluster
	if err = c.Delete(context.TODO(), cluster, &client.DeleteOptions{}); err != nil {
		return false, err
	}

	// Try and wait for deletion
	retry := time.Duration(10 * time.Second)
	timeout := time.Duration(time.Hour)
	err = WaitForDeletion(c, cluster, retry, timeout)
	if err != nil {
		return false, err
	}

	//if we are here we're okay
	return true, nil
}

// CopyAzureSecrets
func MoveAzureSecrets(src string, dest string) (bool, error) {
	// Create clients
	log.Info("creating clients for moving azure secrets")
	srcclient, err := clientcmd.BuildConfigFromFlags("", src)
	if err != nil {
		return false, err
	}
	srcclientset, err := kubernetes.NewForConfig(srcclient)
	if err != nil {
		return false, err
	}
	destclient, err := clientcmd.BuildConfigFromFlags("", dest)
	if err != nil {
		return false, err
	}
	destclientset, err := kubernetes.NewForConfig(destclient)
	if err != nil {
		return false, err
	}
	log.Info("src: " + src)
	log.Info("dest: " + dest)

	// Get and move secret
	log.Info("moving secret")
	secret, err := srcclientset.CoreV1().Secrets("default").Get(context.TODO(), "cluster-identity-secret", metav1.GetOptions{})
	if err != nil {
		return false, err
	}
	log.Info("got secret")
	secret.ObjectMeta.ResourceVersion = ""

	_, err = destclientset.CoreV1().Secrets("default").Create(context.TODO(), secret, metav1.CreateOptions{})
	if err != nil {
		return false, err
	}
	log.Info("copied secret")
	// Get and move AzureIdentity
	log.Info("moving azure identity")
	dynamicsrc := dynamic.NewForConfigOrDie(srcclient)
	dynamicdest := dynamic.NewForConfigOrDie(destclient)

	resourceId := schema.GroupVersionResource{
		Group:    "infrastructure.cluster.x-k8s.io",
		Version:  "v1beta1",
		Resource: "azureclusteridentities",
	}

	azureIdentity, err := dynamicsrc.Resource(resourceId).Namespace("default").Get(context.TODO(), "cluster-identity", metav1.GetOptions{})
	if err != nil {
		return false, err
	}

	azureIdentity.SetResourceVersion("")

	_, err = dynamicdest.Resource(resourceId).Namespace("default").Create(context.TODO(), azureIdentity, metav1.CreateOptions{})
	if err != nil {
		return false, err
	}
	log.Info("copied azure identity")
	return true, nil
}

// MoveMgmtCluster moves the management cluster from src kubeconfig to dest kubeconfig
func MoveMgmtCluster(src string, dest string, capiImplementation string) (bool, error) {
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
	// Create clientset for dest
	destclient, err := clientcmd.BuildConfigFromFlags("", dest)
	if err != nil {
		return false, err
	}
	destclientset, err := kubernetes.NewForConfig(destclient)
	if err != nil {
		return false, err
	}
	// Get the secret and base64 encode it (you'd think it would come encoded but it doesn't)
	capNamespace := capiImplementation + "-system"
	capSecretName := capiImplementation + "-manager-bootstrap-credentials"

	// init the dest cluster
	if capiImplementation == "capa" {
		secret, err := srcclientset.CoreV1().Secrets(capNamespace).Get(context.TODO(), capSecretName, metav1.GetOptions{})
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
		_, err = c.Init(capiclient.InitOptions{
			Kubeconfig:              capiclient.Kubeconfig{Path: dest},
			InfrastructureProviders: []string{"aws"},
		})

		if err != nil {
			return false, err
		}
	} else if capiImplementation == "capz" {
		log.Info("setting op CAPZ on target cluster")
		_, err = c.Init(capiclient.InitOptions{
			Kubeconfig:              capiclient.Kubeconfig{Path: dest},
			InfrastructureProviders: []string{"azure"},
		})
		// Check to see if it's rolled out, if not then wait 20 seconds and check again. Stop after 15x
		counter := 0
		for runs := 15; counter <= runs; counter++ {
			capzClient := destclientset.AppsV1().Deployments("capz-system")
			if counter > runs {
				return false, errors.New("CAPI Controller took too long to roll out")
			}
			capzDeployment, err := capzClient.Get(context.TODO(), "capz-controller-manager", metav1.GetOptions{})
			if err != nil {
				return false, err
			}

			availableReplicas := capzDeployment.Status.AvailableReplicas
			if availableReplicas > int32(0) {
				time.Sleep(20 * time.Second)
				break
			}
			time.Sleep(20 * time.Second)
		}
		if err != nil {
			return false, err
		}
		_, err = MoveAzureSecrets(src, dest)
		if err != nil {
			return false, err
		}
	}

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
	//os.Unsetenv("AWS_B64ENCODED_CREDENTIALS")

	// if we're here we must be okay
	return true, nil
}

// WaitForDeletion waits for the resouce to be deleted
//func WaitForDeletion(dynclient client.Client, obj runtime.Object, retryInterval, timeout time.Duration) error {
func WaitForDeletion(dynclient client.Client, obj client.Object, retryInterval, timeout time.Duration) error {
	key := client.ObjectKeyFromObject(obj)

	//kind := obj.GetObjectKind().GroupVersionKind().Kind
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	err := wait.Poll(retryInterval, timeout, func() (done bool, err error) {
		err = dynclient.Get(ctx, key, obj)
		if apierrors.IsNotFound(err) {
			return true, nil
		}
		if err != nil {
			return false, err
		}
		//log.Infof("Waiting for %s %s to be deleted\n", kind, key)
		return false, nil
	})
	if err != nil {
		return err
	}
	return nil
}
