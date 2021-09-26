package utils

import (
	"context"
	"io/ioutil"
	"os"
	"os/exec"
	"text/template"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/cluster-api/cmd/clusterctl/client"
)

// CheckPreReqs() checks to see if you have the proper CLI tools installed
func CheckPreReqs() (bool, error) {
	// This is the expected cli utils we expect you to haveinstalled
	log.Info("Running checks")
	cliUtils := [6]string{"kubectl", "kind", "clusterawsadm", "docker", "clusterctl", "git"}
	for _, cli := range cliUtils {
		_, err := exec.LookPath(cli)
		if err != nil {
			return false, err
		}
	}
	return true, nil
}

// CreateWorkDir creates a temp dir to store all the things we need
func CreateWorkDir() (string, error) {
	// Genarate a temp directory for our work
	dir, err := ioutil.TempDir("/tmp", "gokp")

	// check for errors
	if err != nil {
		return "", err
	}

	// return the dirname and no error
	return dir, nil

}

// WriteTemplate is a generic template writing mechanism
func WriteTemplate(tpl string, fileToCreate string, vars interface{}) (bool, error) {
	tmpl := template.Must(template.New("").Parse(tpl))
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

// WriteYamlOutput writes YAML to the specified file path
func WriteYamlOutput(printer client.YamlPrinter, file string) error {
	f, err := os.Create(file)
	if err != nil {
		return err
	}
	defer f.Close()

	yaml, err := printer.Yaml()
	yaml = append(yaml, '\n')
	if err != nil {
		return err
	}

	if _, err := f.Write(yaml); err != nil {
		return err
	}
	return nil
}

// DeploymentAvailableReplicas just returns the available replicas
func DeploymentAvailableReplicas(clusterInstallConfig *rest.Config, namespace string, deployment string) (int32, error) {

	clusterInstallClientSet, err := kubernetes.NewForConfig(clusterInstallConfig)
	if err != nil {
		return 0, err
	}

	dClient := clusterInstallClientSet.AppsV1().Deployments(namespace)
	depl, err := dClient.Get(context.TODO(), deployment, metav1.GetOptions{})
	if err != nil {
		return 0, err
	}

	return depl.Status.AvailableReplicas, nil
}
