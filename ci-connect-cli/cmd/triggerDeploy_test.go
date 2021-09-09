package cmd

import (
	"bytes"
	"github.com/stretchr/testify/assert"
	"os"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/spf13/cobra"
)

func setupEnv() {
	os.Clearenv()
	os.Setenv("GIT_REPO", "test_git_repo")
	os.Setenv("GIT_USER", "test_git_user")
	os.Setenv("GIT_TOKEN", "test_git_token")
	os.Setenv("SERVICE", "test_service")
}

func createDeploymentMock(t *testing.T) *MockDeployment {
	mockCtrl := gomock.NewController(t)
	return NewMockDeployment(mockCtrl)
}

func setupRootCmdWithDeploymentMock(t *testing.T) {
	deployment := createDeploymentMock(t)
	deployment.EXPECT().RunDeployment().Times(1)
	triggerDeployCmd := NewTriggerDeployCmd(deployment)
	rootCmd.ResetCommands()
	rootCmd.AddCommand(triggerDeployCmd)
}

func executeCommand(command *cobra.Command, args ...string) (output string, err error) {
	buf := new(bytes.Buffer)
	command.SetOut(buf)
	command.SetErr(buf)
	command.SetArgs(args)
	err = command.Execute()
	return buf.String(), err
}

func assertCommandNoOutputAndError(t *testing.T, output string, err error) {
	if output != "" {
		t.Errorf("Unexpected output: %v", output)
	}
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestTriggerDeployment_RequiredFlags(t *testing.T) {
	os.Clearenv()
	_, err := executeCommand(rootCmd, "trigger-deployment")
	assert.Equal(t, "required flag(s) \"git-repo\", \"git-token\", \"git-user\", \"service\", \"workspace\" not set", err.Error())
}

func TestTriggerDeployment_FlagByEnv(t *testing.T) {
	setupEnv()
	setupRootCmdWithDeploymentMock(t)
	os.Setenv("WORKSPACE", "test_workspace")

	output, err := executeCommand(rootCmd, "trigger-deployment")
	assertCommandNoOutputAndError(t, output, err)
	assert.Equal(t, "test_workspace", *triggerDeployParams.Workspace)
}

func TestTriggerDeployment_EnvOverwrite(t *testing.T) {
	setupEnv()
	setupRootCmdWithDeploymentMock(t)
	os.Setenv("WORKSPACE", "test_workspace")

	output, err := executeCommand(rootCmd, "trigger-deployment", "--workspace", "test_workspace_overwrite")
	assertCommandNoOutputAndError(t, output, err)
	assert.Equal(t, "test_workspace_overwrite", *triggerDeployParams.Workspace)
}
