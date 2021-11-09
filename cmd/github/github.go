package github

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"io/ioutil"
	"os"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	plumbingssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/google/go-github/v39/github"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
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
		return false, "", err
	}

	// Create an SSHKeypair for the repo.
	publicKeyBytes, err := generateSSHKeypair(*name, workdir)
	if err != nil {
		return false, "", err
	}

	// upload public sshkey as a deploy key
	err = uploadDeployKey(publicKeyBytes, repo.GetOwner().GetLogin(), *name, client)
	if err != nil {
		return false, "", err
	}

	// Get the remote URL and set the name of the local copy
	//repoUrl := repo.GetCloneURL()
	repoUrl := repo.GetSSHURL()
	localRepo := workdir + "/" + *name

	// Maksure the localRepo is there
	os.MkdirAll(localRepo, 0755)

	// Read sshkey to do the clone
	privateKeyFile := workdir + "/" + *name + "_rsa"
	authKey, err := plumbingssh.NewPublicKeysFromFile("git", privateKeyFile, "")
	if err != nil {
		return false, "", err
	}

	// Clone the repo locally in the working dir (as localRepo)
	_, err = git.PlainClone(localRepo, false, &git.CloneOptions{
		URL:  repoUrl,
		Auth: authKey,
		/*
			Auth: &http.BasicAuth{
				Username: "unused",
				Password: token,
			},
		*/
	})

	if err != nil {
		return false, "", err
	}

	log.Info("Successfully created new repo: ", repoUrl)
	return true, repoUrl, nil
}

// CommitAndPush commits and pushes changes to a github repo that has been changed locally
func CommitAndPush(dir string, privateKeyFile string, msg string) (bool, error) {
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

	// Read sshkey to do the clone
	authKey, err := plumbingssh.NewPublicKeysFromFile("git", privateKeyFile, "")
	if err != nil {
		return false, err
	}

	//Push to repo
	err = repo.Push(&git.PushOptions{
		RemoteName: "origin",
		Auth:       authKey,
		/*
			Auth: &http.BasicAuth{
				Username: "unused",
				Password: token,
			},
		*/
	})

	if err != nil {
		return false, err
	}

	// If we're here, we should be good
	log.Info("Successfully pushed commit")

	return true, nil
}

// generateSSHKeypair generates an sshkeypair to use as a deploykey on Github
func generateSSHKeypair(clustername string, workdir string) ([]byte, error) {
	key := workdir + "/" + clustername + "_rsa"
	savePrivateFileTo := key
	savePublicFileTo := key + ".pub"
	bitSize := 4096

	privateKey, err := generatePrivateKey(bitSize)
	if err != nil {
		return nil, err
	}

	publicKeyBytes, err := generatePublicKey(&privateKey.PublicKey)
	if err != nil {
		return nil, err
	}

	privateKeyBytes := encodePrivateKeyToPEM(privateKey)

	err = writeKeyToFile(privateKeyBytes, savePrivateFileTo)
	if err != nil {
		return nil, err
	}

	err = writeKeyToFile([]byte(publicKeyBytes), savePublicFileTo)
	if err != nil {
		return nil, err
	}
	return publicKeyBytes, nil
}

// generatePrivateKey creates a RSA Private Key of specified byte size
func generatePrivateKey(bitSize int) (*rsa.PrivateKey, error) {
	// Private Key generation
	privateKey, err := rsa.GenerateKey(rand.Reader, bitSize)
	if err != nil {
		return nil, err
	}

	// Validate Private Key
	err = privateKey.Validate()
	if err != nil {
		return nil, err
	}

	return privateKey, nil
}

// encodePrivateKeyToPEM encodes Private Key from RSA to PEM format
func encodePrivateKeyToPEM(privateKey *rsa.PrivateKey) []byte {
	// Get ASN.1 DER format
	privDER := x509.MarshalPKCS1PrivateKey(privateKey)

	// pem.Block
	privBlock := pem.Block{
		Type:    "RSA PRIVATE KEY",
		Headers: nil,
		Bytes:   privDER,
	}

	// Private key in PEM format
	privatePEM := pem.EncodeToMemory(&privBlock)

	return privatePEM
}

// generatePublicKey take a rsa.PublicKey and return bytes suitable for writing to .pub file. Returns in the format "ssh-rsa ..."
func generatePublicKey(privatekey *rsa.PublicKey) ([]byte, error) {
	publicRsaKey, err := ssh.NewPublicKey(privatekey)
	if err != nil {
		return nil, err
	}

	pubKeyBytes := ssh.MarshalAuthorizedKey(publicRsaKey)

	return pubKeyBytes, nil
}

// writePemToFile writes keys to a file
func writeKeyToFile(keyBytes []byte, saveFileTo string) error {
	err := ioutil.WriteFile(saveFileTo, keyBytes, 0600)
	if err != nil {
		return err
	}

	return nil
}

// uploadDeployKey uploads deploykey to GitHub
func uploadDeployKey(publicKeyBytes []byte, repoOwner string, name string, client *github.Client) error {
	// Set up the github key object based on the key given to use as a []byte
	mykey := string(publicKeyBytes)
	key := &github.Key{
		Key: &mykey,
	}

	// upload the deploykey to the repo
	_, _, err := client.Repositories.CreateKey(context.TODO(), repoOwner, name, key)
	if err != nil {
		return err
	}

	// if we're here we should be okay
	return nil
}
