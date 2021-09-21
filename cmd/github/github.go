package github

import (
	"context"

	"github.com/google/go-github/v39/github"
	log "github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
)

// CreateRepo taks a name, token, and a private request and creates a repository on GitHub
func CreateRepo(name *string, token string, private *bool) (bool, error) {
	desc := "GitOps repo Cluster " + *name
	description := &desc
	log.Info("Creating repo: ", *name)

	// display if a private repo was requested
	if *private {
		log.Info("Private repo requested")
	}

	// create the repo
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)

	r := &github.Repository{Name: name, Private: private, Description: description}
	repo, _, err := client.Repositories.Create(ctx, "", r)
	if err != nil {
		log.Fatal(err)
		return false, err
	}
	log.Info("Successfully created new repo: ", repo.GetName())
	return true, nil
}
