package github

import (
	"context"
	"os"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/google/go-github/v39/github"
	log "github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
)

// CreateRepo taks a name, token, and a private request and creates a repository on GitHub
func CreateRepo(name *string, token string, private *bool, workdir string) (bool, string, error) {
	desc := "GitOps repo Cluster " + *name
	description := &desc
	autoInit := true
	log.Info("Creating repo for: ", *name)

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
		return false, "", err
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
		return false, "", err
	}

	log.Info("Successfully created new repo: ", repoUrl)
	return true, repoUrl, nil
}

func CommitAndPush(dir string, token string, msg string) (bool, error) {
	// Open the dir for commiting
	repo, err := git.PlainOpen(dir)
	if err != nil {
		return false, err
	}

	// create worktree
	worktree, err := repo.Worktree()
	if err != nil {
		return false, err
	}

	// Add all you did to the worktree
	_, err = worktree.Add("cluster")
	if err != nil {
		return false, err
	}

	// verify status
	_, err = worktree.Status()
	if err != nil {
		return false, err
	}

	//Commit
	_, err = worktree.Commit(msg, &git.CommitOptions{
		Author: &object.Signature{
			Name: "gokp-bootstrapper",
			When: time.Now(),
		},
		All: true,
	})
	if err != nil {
		return false, err
	}

	//Push to repo
	err = repo.Push(&git.PushOptions{
		RemoteName: "origin",
		Auth: &http.BasicAuth{
			Username: "unused",
			Password: token,
		},
	})

	if err != nil {
		return false, err
	}

	// If we're here, we should be good
	log.Info("Successfully pushed commit")

	return true, nil
}
