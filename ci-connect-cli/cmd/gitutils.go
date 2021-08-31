package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
)

type GitPushError struct {
	Msg string
	Err error
}

func (e GitPushError) Error() string {
	return fmt.Sprintf("%v: %v", e.Msg, e.Err)
}

func (e GitPushError) Is(target error) bool {
	_, ok := target.(GitPushError)
	return ok
}

type gitRepositoryConfig struct {
	remoteURI *string
	user      *string
	token     *string
}

type gitCommitOptions struct {
	commitMessage string
	tag           string
	tagMessage    string
}

func (repositoryConfig *gitRepositoryConfig) CheckOutGitRepo(dir string) (*git.Repository, error) {
	authentication := &http.BasicAuth{
		Username: *repositoryConfig.user,
		Password: *repositoryConfig.token,
	}

	cloneOptions := git.CloneOptions{
		URL:          *repositoryConfig.remoteURI,
		Auth:         authentication,
		SingleBranch: true,
	}

	repo, err := git.PlainClone(dir, false, &cloneOptions)
	if err != nil {
		return nil, fmt.Errorf("Could not checkout "+*repositoryConfig.remoteURI+"/master", err)
	}

	return repo, nil
}

func (repositoryConfig *gitRepositoryConfig) CommitAndPushGitRepo(repository *git.Repository, deploymentConfig DeploymentConfig, gitCommitOptions gitCommitOptions, ignoreDuplicateGitTag bool) error {
	authentication := &http.BasicAuth{
		Username: *repositoryConfig.user,
		Password: *repositoryConfig.token,
	}

	commitOptions := git.CommitOptions{
		Author: &object.Signature{
			Name:  deploymentConfig.GitConfig.UserName,
			Email: deploymentConfig.GitConfig.UserEmail,
			When:  time.Now(),
		},
	}

	w, err := repository.Worktree()
	if err != nil {
		return fmt.Errorf("Could not set worktree: %v", err)
	}

	// go-git can't stage deleted files https://github.com/src-d/go-git/issues/1268
	cmd := exec.Command("git", "add", ".")
	cmd.Dir = w.Filesystem.Root()
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("Could not add files: %v", err)
	}

	_, err = w.Commit(gitCommitOptions.commitMessage, &commitOptions)
	if err != nil {
		return fmt.Errorf("Could not commit: %v", err)
	}

	h, err := repository.Head()

	if gitCommitOptions.tag != "" {
		repository.CreateTag(gitCommitOptions.tag, h.Hash(), &git.CreateTagOptions{
			Tagger: &object.Signature{
				Name:  "KIA CLI",
				Email: "ci-connect@keptn.sh",
				When:  time.Now(),
			},
			Message: gitCommitOptions.tagMessage,
		})
		if err != nil {
			return fmt.Errorf("Couldn't create a tag: %v", err)
		}
	}

	err = repository.Push(&git.PushOptions{
		RemoteName: "origin",
		Auth:       authentication,
		RefSpecs: []config.RefSpec{
			"refs/heads/*:refs/heads/*",
		},
	})
	if err != nil {
		return GitPushError{Msg: "Couldn't push commit", Err: err}
	}

	err = repository.Push(&git.PushOptions{
		RemoteName: "origin",
		Auth:       authentication,
		RefSpecs: []config.RefSpec{
			"refs/tags/*:refs/tags/*",
		},
	})
	if err != nil {
		if !ignoreDuplicateGitTag {
			return GitPushError{Msg: "Couldn't push tag", Err: err}
		} else {
			fmt.Println("Ignoring Duplicate Git Tag " + gitCommitOptions.tag)
		}
	}

	return nil
}

func ConfirmChangesGitRepo(repository *git.Repository) (bool, error) {
	w, err := repository.Worktree()
	if err != nil {
		return false, fmt.Errorf("Could not set worktree: %v", err)
	}

	status, _ := w.Status()
	print(status.String())

	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Print("The above changes will be made, continue (y/n): ")
		response, err := reader.ReadString('\n')
		if err != nil {
			return false, err
		}

		response = strings.ToLower(strings.TrimSpace(response))
		if response == "y" || response == "yes" {
			return true, nil
		} else if response == "n" || response == "no" {
			return false, nil
		}
	}
}
