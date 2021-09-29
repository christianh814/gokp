package utils

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
			log.Warn("Nonfatal: ", err)
			//return false, err
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

// SplitYamls takes a multi-part YAML file and splits it into multiple files in the specified directory splitting on the string given
func SplitYamls(dir string, yaml string, spliton string) error {
	// Read the YAML file into a []byte
	yamlByte, err := ioutil.ReadFile(yaml)

	if err != nil {
		return err
	}

	// Read the []byte into a string
	yml := string(yamlByte)

	// Split the stringafied YAML and split on what was given to us
	files := strings.Split(yml, spliton)

	// Split gives us a slice of string. Let's itterate and split them

	for i, file := range files {
		// create a directory provided to us
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
		// Create the file and name it based on the index together with the name of the file
		newYaml, err := os.Create(dir + "/" + fmt.Sprintf("%02d", i) + "." + filepath.Base(yaml))
		if err != nil {
			return err
		}
		defer newYaml.Close()

		// Write this new filebased on the string given
		newYaml.WriteString(file)
	}

	return nil
}

// DownloadFile will download a url to a local file. It's like WGET
func DownloadFile(file string, url string) (bool, error) {
	// Get the data
	r, err := http.Get(url)
	if err != nil {
		return false, err
	}
	defer r.Body.Close()

	// Create the file to the specific path
	out, err := os.Create(file)
	if err != nil {
		return false, err
	}
	defer out.Close()

	// Write the body to file
	_, err = io.Copy(out, r.Body)
	return false, err
}
