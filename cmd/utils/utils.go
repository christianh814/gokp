package utils

import (
	"os/exec"

	log "github.com/sirupsen/logrus"
)

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
