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
func ExportClusterYaml(capicfg string, repodir string) (bool, error) {
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

	// Loop through the cluster scoped API resources and export them to <repodir>/<clustername>/core/cluster dir
	for _, car := range clusterApiResouces {
		if car.APIResource.Name == "componentstatuses" || car.APIResource.Name == "namespaces" || car.APIResource.Name == "certificatesigningrequests" || car.APIResource.Name == "podsecuritypolicies" {
			continue
		}
		// export the yaml
		_, err = exportClusterScopedYaml(dynamicClient, car, e, repodir+"/core"+"/cluster")
		if err != nil {
			return false, err
		}
	}

	// Create kustomize file based on the YAMLs created
	dirGlob := repodir + "/core" + "/cluster/*.yaml"
	clusterScopedYamlFiles, err := filepath.Glob(dirGlob)
	if err != nil {
		return false, err
	}

	if len(clusterScopedYamlFiles) == 0 {
		return false, errors.New("no YAML Files found at: " + dirGlob)
	}
	// generate the kustomization.yaml file based on the template
	cskf := struct {
		ClusterScopedYamls []string
	}{
		ClusterScopedYamls: clusterScopedYamlFiles,
	}
	_, err = WriteTemplateWithFunc(ClusterScopedKustomizeFile, repodir+"/core"+"/cluster/kustomization.yaml", cskf, FuncMap)
	if err != nil {
		return false, err
	}

	// Second, export namespaced scoped api resources
	//CHX

	// If we're here we should be okay
	return true, nil
}

// exportClusterScopedYaml exports all the cluster scoped yaml into the given directory
func exportClusterScopedYaml(client dynamic.Interface, gr GroupResource, e *json.Serializer, dir string) (bool, error) {
	//fmt.Printf(fmt.Sprintf("Querying for %s in %s group\n", gr.APIResource.Name, gr.APIGroupVersion))
	//list, err := client.Resource(schema.GroupVersionResource{Group: gr.APIGroup, Resource: gr.APIResource.Name, Version: gr.APIGroupVersion}).List(context.TODO(), metav1.ListOptions{})
	list, err := client.Resource(schema.GroupVersionResource{Group: gr.APIGroup, Resource: gr.APIResource.Name, Version: gr.APIVersion}).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return false, err
	}

	os.MkdirAll(dir, 0755)

	for _, listItem := range list.Items {
		listItem.SetGroupVersionKind(schema.GroupVersionKind{Group: gr.APIResource.Group, Kind: gr.APIResource.Kind, Version: gr.APIVersion})
		metadata := listItem.Object["metadata"].(map[string]interface{})

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
