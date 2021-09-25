package utils

import (
	"io/ioutil"
	"os"
	"os/exec"
	"text/template"

	log "github.com/sirupsen/logrus"
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
