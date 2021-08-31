package cmd

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"path"

	"github.com/spf13/cobra"
)

const (
	MaxDeploymentRepetitionsOnGitPushError int = 10
)

//go:generate mockgen -source=triggerDeploy.go -destination=deployment_mock.go -package=cmd Deployment

type Deployment interface {
	RunDeployment() error
}

type deploymentImpl struct {
}

type ShipyardConfig struct {
	Spec SpecConfig `yaml:"spec,omitempty"`
}

type SpecConfig struct {
	Stages []StageConfig `yaml:"stages,omitempty"`
}

type StageConfig struct {
	Name      string           `yaml:"name,omitempty"`
	Sequences []SequenceConfig `yaml:"sequences,omitempty"`
}

type SequenceConfig struct {
	Name string `yaml:"name,omitempty"`
}

type DeploymentConfig struct {
	Services  []ServiceConfig `yaml:"services"`
	GitConfig GitConfig       `yaml:"git_config"`
}

type DeploymentManifest struct {
	Metadata DeploymentMetadata `yaml:"metadata"`
}

type DeploymentMetadata struct {
	ImageVersion string `yaml:"imageVersion"`
	GitCommit    string `yaml:"gitCommit"`
	Author       string `yaml:"author"`
}

type ServiceConfig struct {
	ServiceName            string `yaml:"name"`
	ChartBaseDirectory     string `yaml:"chart_base,omitempty"`
	UpdateHelmDependencies bool   `yaml:"updateHelmDependencies,omitempty"`
	UseChartVersion        bool   `yaml:"useChartVersion,omitempty"`
	UseChartAppVersion     bool   `yaml:"useChartAppVersion,omitempty"`
	IgnoreDuplicateGitTag  bool   `yaml:"ignoreDuplicateGitTag,omitempty"`
}

type GitConfig struct {
	UserEmail string `yaml:"user_email"`
	UserName  string `yaml:"user_name"`
}

type TriggerDeployCmdParams struct {
	BaseDirectory string
	Workspace     *string
	Service       *string
	CommitMessage *string
	Version       *string
	Sequence      *string
	Stage         *string
	DryRun        *bool
	Repository    gitRepositoryConfig
}

type KeptnConfig struct {
	Metadata KeptnConfigMeta `yaml:"metadata,omitempty"`
	Services []KeptnService  `yaml:"services,omitempty"`
}

type KeptnConfigMeta struct {
	Branch string `yaml:"initbranch,omitempty"`
}

type KeptnService struct {
	Name              string `yaml:"name,omitempty"`
	DeploymentTrigger string `yaml:"triggerevent"`
}

var triggerDeployParams *TriggerDeployCmdParams

func (deployment *deploymentImpl) RunDeployment() error {
	dir, _ := ioutil.TempDir("", "temp_dir_master")
	fmt.Println("Output Directory: " + dir)

	conf := DeploymentConfig{}
	err := conf.GetCiConfig(triggerDeployParams.BaseDirectory + "/ci_config.yaml")
	if err != nil {
		log.Fatal(err)
	}

	repo, err := triggerDeployParams.Repository.CheckOutGitRepo(dir)
	if err != nil {
		log.Fatal(err)
	}

	err = conf.UpdateRepository(dir)
	if err != nil {
		log.Fatal(err)
	}

	if !*triggerDeployParams.DryRun {
		if *triggerDeployParams.CommitMessage == "" {
			*triggerDeployParams.CommitMessage = "Update service " + *triggerDeployParams.Service + " to version " + *triggerDeployParams.Version
		}

		gitCommitOptions := gitCommitOptions{
			commitMessage: *triggerDeployParams.CommitMessage,
			tag:           *triggerDeployParams.Service + "-" + *triggerDeployParams.Version,
			tagMessage:    "The Version " + *triggerDeployParams.Service + "-" + *triggerDeployParams.Version,
		}

		ignoreDuplicateGitTag := false
		for _, service := range conf.Services {
			if service.ServiceName == *triggerDeployParams.Service && service.IgnoreDuplicateGitTag {
				ignoreDuplicateGitTag = service.IgnoreDuplicateGitTag
			}
		}
		err = triggerDeployParams.Repository.CommitAndPushGitRepo(repo, conf, gitCommitOptions, ignoreDuplicateGitTag)
		if err != nil {
			return err
		}
	} else {
		fmt.Println("Would Perform Git Update Now")
	}

	return nil
}

func NewTriggerDeployCmd(deployment Deployment) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "trigger-deployment",
		Short: `Push keptn configuration files and trigger a deployment with the keptn git operator`,
		Long: `Copies keptn service configuration files from the $WORKSPACE/.keptn directory into a local clone
of the keptn configuration repository, creates metadata files, commits and pushes the changes.
The commit will be tagged with $SERVICE-$VERSION, the default commit message can be overwritten.

The created commit will have all the necessary metadata for the keptn git operator to do its work.

All flags can also be set with environment variables instead e.g. 
* --workspace <workspace> or 
* export WORKSPACE=<workspace>`,
		RunE: func(cmd *cobra.Command, args []string) error {
			triggerDeployParams.BaseDirectory = path.Join(*triggerDeployParams.Workspace, ".keptn")
			deploymentRepetitions := 0
			err := deployment.RunDeployment()
			for err != nil && deploymentRepetitions < MaxDeploymentRepetitionsOnGitPushError {
				if errors.Is(err, GitPushError{}) {
					log.Println(err)
					deploymentRepetitions = deploymentRepetitions + 1
					log.Printf("Deployment repetition - %v", deploymentRepetitions)
					err = deployment.RunDeployment()
				} else {
					log.Fatal(err)
				}
			}
			return nil
		},
	}

	triggerDeployParams = &TriggerDeployCmdParams{}
	triggerDeployParams.Workspace = cmd.Flags().StringP("workspace", "w", "", "The path to the directory where the .keptn directory resides in")
	triggerDeployParams.Version = cmd.Flags().StringP("version", "x", "", "The version of the deployment")
	triggerDeployParams.Service = cmd.Flags().StringP("service", "s", "", "The service which should be deployed")
	triggerDeployParams.CommitMessage = cmd.Flags().StringP("commit-message", "c", "", "The commit message for the deployment")
	triggerDeployParams.Stage = cmd.Flags().StringP("stage", "g", "", "Which stage should the triggerevent use, overwrites value from shipyard config")
	triggerDeployParams.Sequence = cmd.Flags().StringP("sequence", "q", "", "Which sequence should the triggerevent use, overwrites value from shipyard config")
	triggerDeployParams.DryRun = cmd.Flags().BoolP("dry-run", "d", false, "Perform a dry-run")

	cmd.MarkFlagRequired("service")
	cmd.MarkFlagRequired("workspace")
	prepareGitRepoCmd(&triggerDeployParams.Repository, cmd)

	return cmd
}

func init() {
	deployment := &deploymentImpl{}
	triggerDeployCmd := NewTriggerDeployCmd(deployment)
	rootCmd.AddCommand(triggerDeployCmd)
}
