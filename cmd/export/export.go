package export

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/kubectl/pkg/scheme"
)

var FuncMap = template.FuncMap{
	"Myfp": Myfp,
}

// ExportClusterYaml exports the given clusters YAML into the directory
func ExportClusterYaml(capicfg string, repodir string, gitOpsController string) (bool, error) {
	/* repodir == workdir + clustername */

	//Create client and dynamtic client
	client, err := newClient(capicfg)
	if err != nil {
		return false, err
	}

	dynamicClient, err := newDynamicClient(capicfg)
	if err != nil {
		return false, err
	}

	// Create a YAML serializer
	e := json.NewYAMLSerializer(json.DefaultMetaFactory, nil, nil)

	// First get the cluster scoped api resources
	isNamespaced := false
	clusterApiResouces, err := getApiResources(client, isNamespaced)
	if err != nil {
		return false, err
	}

	// Loop through the cluster scoped API resources and export them to <repodir>/core/cluster dir
	for _, car := range clusterApiResouces {
		// Skip over things we won't or can't export
		if car.APIResource.Name == "componentstatuses" ||
			car.APIResource.Name == "namespaces" ||
			car.APIResource.Name == "certificatesigningrequests" ||
			car.APIResource.Name == "tokenreviews" ||
			car.APIResource.Name == "selfsubjectaccessreviews" ||
			car.APIResource.Name == "selfsubjectrulesreviews" ||
			car.APIResource.Name == "subjectaccessreviews" ||
			car.APIResource.Name == "ipamblocks" ||
			car.APIResource.Name == "ipamhandles" ||
			car.APIResource.Name == "ipamconfigs" ||
			car.APIResource.Name == "podsecuritypolicies" {
			continue
		}
		// export the yaml
		_, err = exportClusterScopedYaml(dynamicClient, car, e, repodir+"/cluster"+"/core/cluster", "NOT-NAMESPACED")
		if err != nil {
			return false, err
		}
	}

	// Create kustomize file based on the YAMLs created
	dirGlob := repodir + "/cluster" + "/core/cluster/*.yaml"
	clusterScopedYamlFiles, err := filepath.Glob(dirGlob)
	if err != nil {
		return false, err
	}

	// If there is no yaml files, error
	if len(clusterScopedYamlFiles) == 0 {
		return false, errors.New("no YAML Files found at: " + dirGlob)
	}

	// generate the kustomization.yaml file based on the template
	cskf := struct {
		ClusterScopedYamls []string
		GitOpsController   string
	}{
		ClusterScopedYamls: clusterScopedYamlFiles,
		GitOpsController:   gitOpsController,
	}
	_, err = WriteTemplateWithFunc(ClusterScopedKustomizeFile, repodir+"/cluster/core/cluster/kustomization.yaml", cskf, FuncMap)
	if err != nil {
		return false, err
	}

	// Second, export namespaced scoped api resources
	isItNamespaced := true
	namespacedApis, err := getApiResources(client, isItNamespaced)
	if err != nil {
		return false, err
	}

	namespaces, err := client.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return false, err
	}

	// range through every namespace and extract the YAML
	for _, ns := range namespaces.Items {
		outdir := repodir + "/cluster/core/" + ns.Name
		// Get each namespaced api component
		for _, nc := range namespacedApis {
			// get the namespace object for later writing to YAML
			thens, err := client.CoreV1().Namespaces().Get(context.TODO(), ns.Name, metav1.GetOptions{})
			if err != nil {
				return false, err
			}
			thens.SetResourceVersion("")
			thens.SetUID("")
			thens.SetAnnotations(map[string]string{})
			thens.CreationTimestamp.Reset()
			thens.SetSelfLink("")
			thens.SetManagedFields([]metav1.ManagedFieldsEntry{})
			thens.SetFinalizers([]string{})
			thens.SetOwnerReferences([]metav1.OwnerReference{})
			thens.SetGeneration(0)
			thens.Status.Reset()
			addTypeInformationToObject(thens)
			encodeNs := json.NewYAMLSerializer(json.DefaultMetaFactory, nil, nil)

			// skip unneeded resources
			if nc.APIResource.Name == "bindings" ||
				nc.APIResource.Name == "pods" ||
				nc.APIResource.Name == "endpoints" ||
				nc.APIResource.Name == "replicasets" ||
				nc.APIResource.Name == "localsubjectaccessreviews" ||
				nc.APIResource.Name == "selfsubjectrulesreviews" ||
				nc.APIResource.Name == "events" ||
				nc.APIResource.Name == "endpointslices" {
				continue
			}
			// export every namespaced resource in the namespace
			_, err = exportClusterScopedYaml(dynamicClient, nc, e, outdir, ns.Name)
			if err != nil {
				return false, err
			}

			// write out the namespace YAML
			tnf, err := os.Create(outdir + "/namespace-" + ns.Name + ".yaml")
			if err != nil {
				return false, err
			}
			err = encodeNs.Encode(thens, tnf)
			if err != nil {
				return false, err
			}
			tnf.Close()
		}

		// Create kustomize file based on the YAMLs created
		dirGlob := repodir + "/cluster" + "/core/" + ns.Name + "/*.yaml"
		nsScopedYamlFiles, err := filepath.Glob(dirGlob)
		if err != nil {
			return false, err
		}

		// If there is no yaml files, error
		if len(nsScopedYamlFiles) == 0 {
			return false, errors.New("no YAML Files found at: " + dirGlob)
		}

		// generate the kustomization.yaml file based on the template
		nskf := struct {
			NsScopedYamls    []string
			GitOpsController string
		}{
			NsScopedYamls:    nsScopedYamlFiles,
			GitOpsController: gitOpsController,
		}
		_, err = WriteTemplateWithFunc(NameSpacedScopedKustomizeFile, repodir+"/cluster/core/"+ns.Name+"/kustomization.yaml", nskf, FuncMap)
		if err != nil {
			return false, err
		}
	}

	// If we're here we should be okay
	return true, nil
}

// exportClusterScopedYaml exports all the cluster scoped yaml into the given directory
func exportClusterScopedYaml(client dynamic.Interface, gr GroupResource, e *json.Serializer, dir string, ns string) (bool, error) {
	//fmt.Printf(fmt.Sprintf("Querying for %s in %s group\n", gr.APIResource.Name, gr.APIGroupVersion))
	//list, err := client.Resource(schema.GroupVersionResource{Group: gr.APIGroup, Resource: gr.APIResource.Name, Version: gr.APIGroupVersion}).List(context.TODO(), metav1.ListOptions{})

	/* list, err = client.Resource(schema.GroupVersionResource{Group: gr.APIGroup, Resource: gr.APIResource.Name, Version: gr.APIVersion}).List(context.TODO(), metav1.ListOptions{}) */

	// set up variables for the conditional to follow
	var list *unstructured.UnstructuredList
	var err error

	// filter by namespace
	if ns == "NOT-NAMESPACED" {
		list, err = client.Resource(schema.GroupVersionResource{Group: gr.APIGroup, Resource: gr.APIResource.Name, Version: gr.APIVersion}).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return false, err
		}
	} else {
		list, err = client.Resource(schema.GroupVersionResource{Group: gr.APIGroup, Resource: gr.APIResource.Name, Version: gr.APIVersion}).List(context.TODO(), metav1.ListOptions{FieldSelector: "metadata.namespace=" + ns})
		if err != nil {
			return false, err
		}
	}

	os.MkdirAll(dir, 0755)

	for _, listItem := range list.Items {
		//listItem.SetGroupVersionKind(schema.GroupVersionKind{Group: gr.APIResource.Group, Kind: gr.APIResource.Kind, Version: gr.APIVersion})
		listItem.SetGroupVersionKind(schema.GroupVersionKind{Group: gr.APIResource.Group, Kind: gr.APIResource.Kind, Version: gr.APIGroupVersion})
		metadata := listItem.Object["metadata"].(map[string]interface{})

		itemName := listItem.GetName()

		// We will skip certian objects as they are managed by something else
		if strings.Contains(itemName, "bootstrap-token") ||
			itemName == "cluster-info" ||
			itemName == "calico-config" {
			continue
		}

		// "Generalize" YAML
		delete(metadata, "resourceVersion")
		delete(metadata, "uid")
		delete(metadata, "annotations")
		delete(metadata, "creationTimestamp")
		delete(metadata, "selfLink")
		delete(metadata, "managedFields")
		delete(metadata, "finalizers")
		delete(metadata, "ownerReferences")
		delete(metadata, "generation")
		delete(listItem.Object, "status")

		// Removing because we're just skipping this now, but leaving it in as a comment because it's useful to know
		/*
			if listItem.GetName() == "cluster-info" {
				listItem.SetAnnotations(map[string]string{"argocd.argoproj.io/compare-options": "IgnoreExtraneous"})
			}
		*/

		obj := listItem.DeepCopyObject()

		fileName := fmt.Sprintf("%s-%s", strings.ReplaceAll(strings.ToLower(listItem.GetKind()), ":", "-"), strings.ReplaceAll(strings.ToLower(listItem.GetName()), ":", "-"))
		y, err := os.Create(dir + "/" + fileName + ".yaml")
		if err != nil {
			return false, err
		}
		defer y.Close()

		err = addTypeInformationToObject(obj)
		if err != nil {
			return false, err
		}

		err = e.Encode(obj, y)
		if err != nil {
			return false, err
		}

	}
	return true, nil
}

// addTypeInformationToObject adds any missing fields to the runtime object
func addTypeInformationToObject(obj runtime.Object) error {
	gvks, _, err := scheme.Scheme.ObjectKinds(obj)
	if err != nil {
		return fmt.Errorf("missing apiVersion or kind and cannot assign it; %w", err)
	}

	for _, gvk := range gvks {
		if len(gvk.Kind) == 0 {
			continue
		}
		if len(gvk.Version) == 0 || gvk.Version == runtime.APIVersionInternal {
			continue
		}
		obj.GetObjectKind().SetGroupVersionKind(gvk)
		break
	}

	return nil
}

// newClient returns a kubernetes interface type
func newClient(kubeConfigPath string) (kubernetes.Interface, error) {
	// build the client and return it
	kubeConfig, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(kubeConfig)
}

// newDynamicClient returns a dyamnic kubernetes interface
func newDynamicClient(kubeConfigPath string) (dynamic.Interface, error) {
	// build the dynamic client and return it
	kubeConfig, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	if err != nil {
		return nil, err
	}
	return dynamic.NewForConfig(kubeConfig)
}

// getApiResources returns a []GroupResource and an err
func getApiResources(k kubernetes.Interface, namespaced bool) ([]GroupResource, error) {
	ar, err := k.Discovery().ServerPreferredResources()
	if err != nil {
		return nil, err
	}
	//
	resources := []GroupResource{}
	for _, list := range ar {
		if len(list.APIResources) == 0 {
			continue
		}
		gv, err := schema.ParseGroupVersion(list.GroupVersion)
		if err != nil {
			continue
		}
		for _, resource := range list.APIResources {
			if len(resource.Verbs) == 0 {
				continue
			}
			// filter namespaced
			if namespaced != resource.Namespaced {
				continue
			}
			// filter to resources that support the specified verbs
			resources = append(resources, GroupResource{
				APIGroup:        gv.Group,
				APIGroupVersion: gv.String(),
				APIVersion:      gv.Version,
				APIResource:     resource,
			})
		}
	}
	return resources, nil
}

func Myfp(s string) string {
	return filepath.Base(s)
}

// WriteTemplateWithFunc is a generic template writing mechanism that supports template.FuncMap
func WriteTemplateWithFunc(tpl string, fileToCreate string, vars interface{}, fm template.FuncMap) (bool, error) {
	tmpl := template.Must(template.New("").Funcs(fm).Parse(tpl))

	file, err := os.Create(fileToCreate)
	if err != nil {
		return false, err
	}

	err = tmpl.Execute(file, vars)

	if err != nil {
		file.Close()
		return false, err
	}
	file.Close()
	return true, nil
}
