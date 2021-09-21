package utils

import (
	"io/ioutil"
	"os/exec"

	log "github.com/sirupsen/logrus"
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
