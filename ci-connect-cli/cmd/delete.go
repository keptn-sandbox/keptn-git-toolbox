package cmd

import (
	"fmt"
	"io/ioutil"
	"log"

	"github.com/spf13/cobra"
)

type deleteCmdParams struct {
	service       *string
	commitMessage *string
	dryRun        *bool
	repository    gitRepositoryConfig
}

var deleteParams *deleteCmdParams

func deleteService() {
	dir, _ := ioutil.TempDir("", "temp_dir_master")
	fmt.Println("git clone directory: " + dir)

	repo, err := deleteParams.repository.CheckOutGitRepo(dir)
	if err != nil {
		log.Fatal(err)
	}

	err = DeleteConfiguration(dir, *deleteParams.service)
	if err != nil {
		log.Fatal(err)
	}

	confirmed, err := ConfirmChangesGitRepo(repo)
	if err != nil {
		log.Fatal(err)
	}

	if !confirmed {
		return
	}

	if !*deleteParams.dryRun {
		if *deleteParams.commitMessage == "" {
			*deleteParams.commitMessage = "Delete configuration of service " + *deleteParams.service
		}

		gitCommitOptions := gitCommitOptions{
			commitMessage: *deleteParams.commitMessage,
		}

		conf := DeploymentConfig{
			GitConfig: GitConfig{
				UserEmail: "ci-connect@keptn.sh",
				UserName:  "Keptn CI Connector",
			},
		}

		err = triggerDeployParams.Repository.CommitAndPushGitRepo(repo, conf, gitCommitOptions, false)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		fmt.Println("Would Perform Git Update Now")
	}
}

var deleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete keptn configuration files and trigger a deletion with the keptn git operator",
	Long:  `Deletes all configuration files of the specified service in the base and stages directories, as
well as the respective service entry in .keptn/config.yaml. The git operator will then trigger a
deletion of the service in keptn.`,
	Run: func(cmd *cobra.Command, args []string) {
		deleteService()
	},
}

func init() {
	deleteParams = &deleteCmdParams{}
	deleteParams.service = deleteCmd.Flags().StringP("service", "s", "", "The service which should be deleted")
	deleteParams.commitMessage = deleteCmd.Flags().StringP("commit-message", "c", "", "The commit message for the deletion")
	deleteParams.dryRun = deleteCmd.Flags().BoolP("dry-run", "d", false, "Perform a dry-run")

	deleteCmd.MarkFlagRequired("service")
	prepareGitRepoCmd(&deleteParams.repository, deleteCmd)
	rootCmd.AddCommand(deleteCmd)
}
