package cmd

import (
	"github.com/spf13/afero"
	"gopkg.in/yaml.v2"
	"gotest.tools/assert"
	"log"
	"path/filepath"
	"testing"
)

const validShipyardConfig = `
apiVersion: spec.keptn.sh/0.2.0
kind: "Shipyard"
metadata:
  name: "keptn-test"
spec:
  stages:
    - name: "one"
      sequences:
        - name: "rick"
          tasks:
            - name: "never"
            - name: "gonna"
            - name: "give"
            - name: "you"
            - name: "up"
        - name: "astley"
          tasks:
            - name: "never"
            - name: "gonna"
            - name: "let"
            - name: "you"
            - name: "down"
    - name: "two"
      sequences:
        - name: "paul"
          tasks:
            - name: "push"
            - name: "it"
            - name: "to"
            - name: "the"
            - name: "limit"
`

const validKeptnOperatorConfig = `
metadata:
  initbranch: "alderan"

services:
- name: "death-star-as-a-service"
  triggerevent: "sh.keptn.event.alderan.delivery.triggered"
- name: "mega-maid-as-a-service"
  triggerevent: "sh.keptn.event.alderan.delivery.triggered"
`

const deploymentConfig = `
services:
  - name: "death-star-as-a-service"
    chart_base: "myChart"
git_config:
  user_email: keptn@keptn.sh
  user_name: jenkins
`

const deploymentConfig2 = `
services:
  - name: "mega-maid-as-a-service"
    chart_base: "myChart"
git_config:
  user_email: keptn@keptn.sh
  user_name: jenkins
`

func TestReadValidShipyardConfig(t *testing.T) {
	fs := afero.NewMemMapFs()
	createFile(t, fs, "", "shipyard.yaml", validShipyardConfig)

	config, err := readShipyardConfigFromFile(fs, "")

	assert.NilError(t, err)
	assert.Equal(t, len(config.Spec.Stages), 2)

	stage1 := config.Spec.Stages[0]
	stage2 := config.Spec.Stages[1]

	assert.Equal(t, stage1.Name, "one")
	assert.Equal(t, stage2.Name, "two")

	assert.Equal(t, len(stage1.Sequences), 2)
	assert.Equal(t, len(stage2.Sequences), 1)

	assert.Equal(t, stage1.Sequences[0].Name, "rick")
	assert.Equal(t, stage1.Sequences[1].Name, "astley")
	assert.Equal(t, stage2.Sequences[0].Name, "paul")
}

func TestReadValidKeptnOperatorConfig(t *testing.T) {
	fs := afero.NewMemMapFs()
	createFile(t, fs, ".keptn", "config.yaml", validKeptnOperatorConfig)

	config, err := readKeptnOperatorConfigFromFile(fs, "")

	assert.NilError(t, err)
	assert.Equal(t, config.Metadata.Branch, "alderan")
	assert.Equal(t, len(config.Services), 2)

	service1 := config.Services[0]
	service2 := config.Services[1]

	assert.Equal(t, service1.Name, "death-star-as-a-service")
	assert.Equal(t, service1.DeploymentTrigger, "sh.keptn.event.alderan.delivery.triggered")

	assert.Equal(t, service2.Name, "mega-maid-as-a-service") // I wonder who gets this reference :D
	assert.Equal(t, service2.DeploymentTrigger, "sh.keptn.event.alderan.delivery.triggered")
}

func TestCopyHelmChart(t *testing.T) {
	fs := afero.NewMemMapFs()
	createFile(t, fs, "myChart", "Chart.yaml", "My Chart.yaml")
	createFile(t, fs, "myChart/templates", "deployment.yaml", "My Deployment")

	deploymentConfig := createDeploymentConfig(deploymentConfig)

	err := copyHelmChart(fs, deploymentConfig.Services[0], ".keptn/base/death-star-as-a-service/helm")

	assert.NilError(t, err)

	assertFileExists(t, fs, ".keptn/base/death-star-as-a-service/helm/Chart.yaml")
	assertFileExists(t, fs, ".keptn/base/death-star-as-a-service/helm/templates/deployment.yaml")
}

func TestCopyBase(t *testing.T) {
	fs := afero.NewMemMapFs()
	createFile(t, fs, ".keptn/base/death-star-as-a-service/helm/templates", "service.yaml", "My service.yaml")
	createFile(t, fs, ".keptn/base/death-star-as-a-service/locust", "health.py", "My smoke test")

	err := copyBase(fs, ".keptn/base/death-star-as-a-service", "myDirForPush/base/death-star-as-a-service")
	assert.NilError(t, err)

	assertFileExists(t, fs, "myDirForPush/base/death-star-as-a-service/helm/templates/service.yaml")
	assertFileExists(t, fs, "myDirForPush/base/death-star-as-a-service/locust/health.py")
}

func TestCopyStages(t *testing.T) {
	fs := afero.NewMemMapFs()
	createFile(t, fs, ".keptn/stages/dev/death-star-as-a-service/helm/templates", "service.yaml", "My service.yaml")
	createFile(t, fs, ".keptn/stages/dev/death-star-as-a-service/locust", "health.py", "My smoke test")

	deploymentConfig := createDeploymentConfig(deploymentConfig)

	triggerDeployParams.BaseDirectory = ".keptn"

	err := copyStages(fs, "myDirForPush", deploymentConfig.Services[0])
	assert.NilError(t, err)

	assertFileExists(t, fs, "myDirForPush/stages/dev/death-star-as-a-service/helm/templates/service.yaml")
	assertFileExists(t, fs, "myDirForPush/stages/dev/death-star-as-a-service/locust/health.py")
}

func TestDeleteStages(t *testing.T) {
	fs := afero.NewMemMapFs()
	createFile(t, fs, ".keptn/stages/dev/death-star-as-a-service/helm/templates", "service.yaml", "My service.yaml")
	createFile(t, fs, ".keptn/stages/dev/death-star-as-a-service/locust", "health.py", "My smoke test")
	createFile(t, fs, ".keptn/stages/dev/mega-maid-as-a-service/helm/templates", "service.yaml", "My service.yaml")
	createFile(t, fs, ".keptn/stages/dev/mega-maid-as-a-service/locust", "health.py", "My smoke test")
	createFile(t, fs, ".keptn/stages/hardening/mega-maid-as-a-service/helm/templates", "service.yaml", "My service.yaml")
	createFile(t, fs, ".keptn/stages/prod/mega-maid-as-a-service/helm/templates", "service.yaml", "My service.yaml")

	deploymentConfig := createDeploymentConfig(deploymentConfig)
	copyStages(fs, "myDirForPush", deploymentConfig.Services[0])

	deploymentConfig = createDeploymentConfig(deploymentConfig2)
	copyStages(fs, "myDirForPush", deploymentConfig.Services[0])

	err := deleteStages(fs, "myDirForPush", "mega-maid-as-a-service")
	assert.NilError(t, err)

	assertFileExists(t, fs, "myDirForPush/stages/dev/death-star-as-a-service/helm/templates/service.yaml")
	assertFileExists(t, fs, "myDirForPush/stages/dev/death-star-as-a-service/locust/health.py")
	assertFileDoesNotExists(t, fs, "myDirForPush/stages/dev/mega-maid-as-a-service")
	assertFileDoesNotExists(t, fs, "myDirForPush/stages/hardening/mega-maid-as-a-service")
	assertFileDoesNotExists(t, fs, "myDirForPush/stages/prod/mega-maid-as-a-service")
}

func TestDeleteBase(t *testing.T) {
	fs := afero.NewMemMapFs()
	createFile(t, fs, ".keptn/base/death-star-as-a-service/helm/templates", "service.yaml", "My service.yaml")
	createFile(t, fs, ".keptn/base/death-star-as-a-service/locust", "health.py", "My smoke test")
	createFile(t, fs, ".keptn/base/mega-maid-as-a-service/helm/templates", "service.yaml", "My service.yaml")
	createFile(t, fs, ".keptn/base/mega-maid-as-a-service/locust", "health.py", "My smoke test")

	copyBase(fs, ".keptn/base/death-star-as-a-service", "myDirForPush/base/death-star-as-a-service")
	copyBase(fs, ".keptn/base/mega-maid-as-a-service", "myDirForPush/base/mega-maid-as-a-service")

	err := deleteBase(fs, "myDirForPush", "mega-maid-as-a-service")
	assert.NilError(t, err)

	assertFileExists(t, fs, "myDirForPush/base/death-star-as-a-service/helm/templates/service.yaml")
	assertFileExists(t, fs, "myDirForPush/base/death-star-as-a-service/locust/health.py")
	assertFileDoesNotExists(t, fs, "myDirForPush/base/mega-maid-as-a-service")
}

func TestModifyOperatorConfig(t *testing.T) {
	fs := afero.NewMemMapFs()
	createFile(t, fs, ".keptn", "config.yaml", validKeptnOperatorConfig)

	config, _ := readKeptnOperatorConfigFromFile(fs, "")

	err := modifyOperatorConfig(fs, "", config, ServiceConfig{ServiceName: "millennium-falcon-as-a-service"}, "dev", "delivery")
	assert.NilError(t, err)

	config, _ = readKeptnOperatorConfigFromFile(fs, "")
	assert.Equal(t, len(config.Services), 3)

	service := config.Services[2]
	assert.Equal(t, service.Name, "millennium-falcon-as-a-service")
	assert.Equal(t, service.DeploymentTrigger, "sh.keptn.event.dev.delivery.triggered")
}

func TestModifyOperatorConfigUpdateExistingService(t *testing.T) {
	fs := afero.NewMemMapFs()
	createFile(t, fs, ".keptn", "config.yaml", validKeptnOperatorConfig)

	config, _ := readKeptnOperatorConfigFromFile(fs, "")

	err := modifyOperatorConfig(fs, "", config, ServiceConfig{ServiceName: "death-star-as-a-service"}, "dev", "delivery")
	assert.NilError(t, err)

	config, _ = readKeptnOperatorConfigFromFile(fs, "")
	assert.Equal(t, len(config.Services), 2)

	service := config.Services[0]
	assert.Equal(t, service.Name, "death-star-as-a-service")
	assert.Equal(t, service.DeploymentTrigger, "sh.keptn.event.dev.delivery.triggered")
}

func TestDeleteServiceFromOperatorConfig(t *testing.T) {
	fs := afero.NewMemMapFs()
	createFile(t, fs, ".keptn", "config.yaml", validKeptnOperatorConfig)

	config, _ := readKeptnOperatorConfigFromFile(fs, "")

	err := deleteServiceFromOperatorConfig(fs, "", config, "mega-maid-as-a-service")
	assert.NilError(t, err)

	config, _ = readKeptnOperatorConfigFromFile(fs, "")
	assert.Equal(t, len(config.Services), 1)

	service := config.Services[0]
	assert.Equal(t, service.Name, "death-star-as-a-service")
	assert.Equal(t, service.DeploymentTrigger, "sh.keptn.event.alderan.delivery.triggered")
}

func TestGetStageAndSequence(t *testing.T) {
	resetTriggerDeployParams()
	fs := afero.NewMemMapFs()
	createFile(t, fs, "", "shipyard.yaml", validShipyardConfig)

	stage, sequence, err := getStageAndSequence(fs, "")

	assert.NilError(t, err)
	assert.Equal(t, stage, "one")
	assert.Equal(t, sequence, "rick")
}

func TestGetStageAndSequenceNoShipyardConfig(t *testing.T) {
	resetTriggerDeployParams()
	fs := afero.NewMemMapFs()

	_, _, err := getStageAndSequence(fs, "")

	assert.Error(t, err, "Could not find shipyard config")
}

func TestGetStageAndSequenceOverwriteShipyardConfig(t *testing.T) {
	resetTriggerDeployParams()
	fs := afero.NewMemMapFs()
	createFile(t, fs, "", "shipyard.yaml", validShipyardConfig)

	*triggerDeployParams.Stage = "two"
	*triggerDeployParams.Sequence = "paul"
	stage, sequence, err := getStageAndSequence(fs, "")

	assert.NilError(t, err)
	assert.Equal(t, stage, "two")
	assert.Equal(t, sequence, "paul")
}

func TestGetStageAndSequenceOverwriteShipyardConfigInvalidStage(t *testing.T) {
	resetTriggerDeployParams()
	fs := afero.NewMemMapFs()
	createFile(t, fs, "", "shipyard.yaml", validShipyardConfig)

	*triggerDeployParams.Stage = "three"
	_, _, err := getStageAndSequence(fs, "")

	assert.Error(t, err, "Could not find stage 'three' in shipyard.yaml")
}

func TestGetStageAndSequenceOverwriteShipyardConfigInvalidSequence(t *testing.T) {
	resetTriggerDeployParams()
	fs := afero.NewMemMapFs()
	createFile(t, fs, "", "shipyard.yaml", validShipyardConfig)

	*triggerDeployParams.Sequence = "michael"
	_, _, err := getStageAndSequence(fs, "")

	assert.Error(t, err, "Could not find sequence 'michael' in stage 'one' in shipyard.yaml")
}

func resetTriggerDeployParams() {
	*triggerDeployParams.Stage = ""
	*triggerDeployParams.Sequence = ""
}

func assertFileExists(t *testing.T, fs afero.Fs, path string) {
	exists, err := afero.Exists(fs, path)

	assert.Check(t, exists)
	assert.NilError(t, err)
}

func assertFileDoesNotExists(t *testing.T, fs afero.Fs, path string) {
	exists, err := afero.Exists(fs, path)

	assert.Check(t, !exists)
	assert.NilError(t, err)
}

func createFile(t *testing.T, fs afero.Fs, path string, filename string, content string) {

	err := fs.MkdirAll(path, 0700)
	assert.NilError(t, err)

	file, err := fs.Create(filepath.Join(path, filename))
	assert.NilError(t, err)

	_, err = file.WriteString(content)
	assert.NilError(t, err)

	err = file.Close()
	assert.NilError(t, err)
}

func createDeploymentConfig(deploymentConfig string) *DeploymentConfig {
	conf := &DeploymentConfig{}
	err := yaml.Unmarshal([]byte(deploymentConfig), conf)
	if err != nil {
		log.Fatal(err)
	}
	return conf
}
