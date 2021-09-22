package github

import (
	"context"
	"os"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/google/go-github/v39/github"
	log "github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
)

// CreateRepo taks a name, token, and a private request and creates a repository on GitHub
func CreateRepo(name *string, token string, private *bool, workdir string) (bool, error) {
	desc := "GitOps repo Cluster " + *name
	description := &desc
	autoInit := true
	log.Info("Creating repo: ", *name)

	// display if a private repo was requested
	if *private {
		log.Info("Private repo requested")
	}

	// create the repo with the options passed
	//	Name: The name of the repo (in this case we use the name of the cluster passed)
	//	Private: If a private repo should be created
	//	Description: Description as it will appear on GitHub
	//	AutoInit: Initialize the repo with the default Readme
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)

	r := &github.Repository{Name: name, Private: private, Description: description, AutoInit: &autoInit}
	repo, _, err := client.Repositories.Create(ctx, "", r)
	if err != nil {
		log.Fatal(err)
		return false, err
	}

	// Get the remote URL and set the name of the local copy
	repoUrl := repo.GetCloneURL()
	localRepo := workdir + "/" + *name

	// Maksure the localRepo is there
	os.MkdirAll(localRepo, 0755)

	// Clone the repo locally in the working dir (as localRepo)
	_, err = git.PlainClone(localRepo, false, &git.CloneOptions{
		URL: repoUrl,
		Auth: &http.BasicAuth{
			Username: "unused",
			Password: token,
		},
	})

	if err != nil {
		log.Fatal(err)
		return false, err
	}

	log.Info("Successfully created new repo: ", repoUrl)
	return true, nil
}
