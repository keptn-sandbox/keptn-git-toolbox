package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/spf13/afero"
	"gopkg.in/yaml.v2"
)

// UpdateRepository
func (conf *DeploymentConfig) UpdateRepository(dir string) error {
	return conf.doUpdateRepository(afero.NewOsFs(), dir)
}

func (conf *DeploymentConfig) doUpdateRepository(fs afero.Fs, dir string) error {
	stage, sequence, err := getStageAndSequence(fs, dir)
	if err != nil {
		return err
	}

	operatorConfig, err := readKeptnOperatorConfigFromFile(fs, dir)
	if err != nil {
		return err
	}

	for _, service := range conf.Services {
		sourceServicePath := filepath.Join(triggerDeployParams.BaseDirectory, "base", service.ServiceName)
		sourceHelmPath := filepath.Join(sourceServicePath, "helm", service.ServiceName)
		destinationBaseServicePath := filepath.Join(dir, "base", service.ServiceName)

		// copy helm chart from a arbitrary location in the service repository into the base folder in the keptn
		// config repo:
		err = copyHelmChart(fs, service, sourceHelmPath)
		if err != nil {
			return err
		}

		if service.UpdateHelmDependencies {
			err = DependencyUpdate(sourceHelmPath)
			if err != nil {
				return fmt.Errorf("Could not update Dependencies: %s", err)
			}
		}

		err = copyBase(fs, sourceServicePath, destinationBaseServicePath)
		if err != nil {
			return err
		}

		err := copyStages(fs, dir, service)
		if err != nil {
			return err
		}

		err = modifyOperatorConfig(fs, dir, operatorConfig, service, stage, sequence)
		if err != nil {
			return err
		}

		*triggerDeployParams.Version, err = getImageVersion(service, sourceHelmPath)
		if err != nil {
			return err
		}

		err = createDeploymentMetadata(fs, dir, service)
		if err != nil {
			return err
		}
	}
	return nil
}

func createDeploymentMetadata(fs afero.Fs, dir string, service ServiceConfig) error {

	sourceRepo, err := git.PlainOpen(*triggerDeployParams.Workspace)
	if err != nil {
		return fmt.Errorf("could not open git repo: %s", err)
	}

	commit := ""
	author := ""

	head, err := sourceRepo.Head()
	if err != nil {
		return fmt.Errorf("could not get head: %s", err)
	}
	commit = head.Hash().String()
	lastCommit, err := sourceRepo.CommitObject(head.Hash())
	if err != nil {
		return fmt.Errorf("could not get commit: %s", err)
	}
	author = lastCommit.Author.Email

	meta := createDeploymentManifest(commit, author)
	out, err := yaml.Marshal(meta)
	if err != nil {
		return fmt.Errorf("Could not create deployment metadata file: %v", err)
	}

	if _, err := fs.Stat(filepath.Join(dir, "base", service.ServiceName, "metadata")); os.IsNotExist(err) {
		err = fs.MkdirAll(filepath.Join(dir, "base", service.ServiceName, "metadata"), 0744)
		if err != nil {
			return fmt.Errorf("Could not create directory "+dir+"/base/"+service.ServiceName+"/metadata", err)
		}
	}
	err = afero.WriteFile(fs, filepath.Join(dir, "base", service.ServiceName, "metadata", "deployment.yaml"), out, 0644)
	if err != nil {
		return fmt.Errorf("Could not write deployment metadata file in "+dir+"/base/"+service.ServiceName+"/metadata/deployment.yaml", err)
	}
	return nil
}

func modifyOperatorConfig(fs afero.Fs, dir string, operatorConfig KeptnConfig, service ServiceConfig, stage string, sequence string) error {
	deploymentTrigger := fmt.Sprintf("sh.keptn.event.%v.%v.triggered", stage, sequence)
	foundOperatorConfig := false
	updatedDeploymentTrigger := false
	for serviceIndex, operatorService := range operatorConfig.Services {
		if operatorService.Name == service.ServiceName {
			foundOperatorConfig = true
			if operatorService.DeploymentTrigger != deploymentTrigger {
				operatorConfig.Services[serviceIndex].DeploymentTrigger = deploymentTrigger
				updatedDeploymentTrigger = true
			}
			break
		}
	}

	if !foundOperatorConfig {
		operatorConfig.Services = append(operatorConfig.Services, KeptnService{service.ServiceName, deploymentTrigger})
	}

	if !foundOperatorConfig || updatedDeploymentTrigger {
		operatorBytes, err := yaml.Marshal(operatorConfig)
		if err != nil {
			fmt.Println("Could not marshal operator config", err)
		}
		keptnDir := filepath.Join(dir, ".keptn")
		err = fs.MkdirAll(keptnDir, 0775)
		if err != nil {
			return fmt.Errorf("Could not create directory " + dir , err)
		}

		err = afero.WriteFile(fs, filepath.Join(keptnDir, "config.yaml"), operatorBytes, 0644)
		if err != nil {
			return fmt.Errorf("Could not write config file in "+dir+"/.keptn/config.yaml", err)
		}
	}
	return nil
}

func copyStages(fs afero.Fs, dir string, service ServiceConfig) error {
	stages, _ := afero.ReadDir(fs, filepath.Join(triggerDeployParams.BaseDirectory, "stages"))
	for _, stage := range stages {
		if stage.IsDir() {
			sourceDir := filepath.Join(triggerDeployParams.BaseDirectory, "stages", stage.Name(), service.ServiceName)
			if _, err := fs.Stat(sourceDir); err == nil {
				destinationDir := filepath.Join(dir, "stages", stage.Name(), service.ServiceName)
				err = fs.RemoveAll(destinationDir)
				if err != nil {
					return fmt.Errorf("Could not delete "+destinationDir, err)
				}
				err := CopyDir(fs, sourceDir, destinationDir)
				if err != nil {
					return fmt.Errorf("Could not copy "+sourceDir+" to "+destinationDir, err)
				}
			}
		}
	}
	return nil
}

func copyBase(fs afero.Fs, sourceServicePath string, destinationBaseServicePath string) error {

	err := fs.RemoveAll(destinationBaseServicePath)
	if err != nil {
		return fmt.Errorf("Could not delete "+destinationBaseServicePath, err)
	}

	err = CopyDir(fs, sourceServicePath, destinationBaseServicePath)
	if err != nil {
		return fmt.Errorf("Could not copy "+sourceServicePath+" to "+destinationBaseServicePath, err)
	}
	return nil
}

func copyHelmChart(fs afero.Fs, service ServiceConfig, sourceHelmPath string) error {
	if service.ChartBaseDirectory != "" {
		err := fs.RemoveAll(sourceHelmPath)
		if err != nil {
			return fmt.Errorf("Could not delete "+sourceHelmPath, err)
		}
		err = CopyDir(fs, filepath.Join(*triggerDeployParams.Workspace, service.ChartBaseDirectory), sourceHelmPath)
		if err != nil {
			return fmt.Errorf("Could not copy "+filepath.Join(*triggerDeployParams.Workspace, service.ChartBaseDirectory)+" to "+sourceHelmPath, err)
		}
	}
	return nil
}

func readKeptnOperatorConfigFromFile(fs afero.Fs, dir string) (KeptnConfig, error) {

	// get operator config
	operatorConfig := KeptnConfig{}

	operatorConfigFile, err := afero.ReadFile(fs, filepath.Join(dir, ".keptn", "config.yaml"))
	if err != nil {
		fmt.Println("Could not find Operator Configuration File, will create a new one")
	}

	err = yaml.Unmarshal(operatorConfigFile, &operatorConfig)
	if err != nil {
		return KeptnConfig{}, fmt.Errorf("Could not unmarshal operator config")
	}
	return operatorConfig, nil
}

func readShipyardConfigFromFile(fs afero.Fs, dir string) (ShipyardConfig, error) {
	shipyardConfig := ShipyardConfig{}

	shipyardConfigFile, err := afero.ReadFile(fs, filepath.Join(dir, "shipyard.yaml"))
	if err != nil {
		return ShipyardConfig{}, fmt.Errorf("Could not find shipyard config")
	}

	err = yaml.Unmarshal(shipyardConfigFile, &shipyardConfig)
	if err != nil {
		return ShipyardConfig{}, fmt.Errorf("Could not unmarshal shipyard config")
	}
	return shipyardConfig, nil
}

func createDeploymentManifest(gitCommit string, author string) DeploymentManifest {
	return DeploymentManifest{
		Metadata: DeploymentMetadata{
			ImageVersion: *triggerDeployParams.Version,
			GitCommit:    gitCommit,
			Author:       author,
		},
	}
}

func (conf *DeploymentConfig) GetCiConfig(config string) error {
	configFile, err := ioutil.ReadFile(config)
	if err != nil {
		return fmt.Errorf("Could not read CI Configuration file from "+config, err)
	}
	err = yaml.Unmarshal(configFile, conf)
	if err != nil {
		return fmt.Errorf("Could not unmarshal CI Configuration file "+config, err)
	}
	return nil
}

func getImageVersion(service ServiceConfig, sourceHelmPath string) (string, error) {

	if *triggerDeployParams.Version == "" {
		if service.UseChartAppVersion {
			return getHelmChartAppVersion(sourceHelmPath)
		} else if service.UseChartVersion {
			return getHelmChartVersion(sourceHelmPath)
		} else {
			return strconv.FormatInt(time.Now().Unix(), 10), nil
		}
	}
	return *triggerDeployParams.Version, nil
}

func DeleteConfiguration(dir string, service string) error {
	return doDeleteConfiguration(afero.NewOsFs(), dir, service)
}

func doDeleteConfiguration(fs afero.Fs, dir string, service string) error {
	operatorConfig, err := readKeptnOperatorConfigFromFile(fs, dir)
	if err != nil {
		return err
	}

	err = deleteBase(fs, dir, service)
	if err != nil {
		return err
	}

	err = deleteStages(fs, dir, service)
	if err != nil {
		return err
	}

	err = deleteServiceFromOperatorConfig(fs, dir, operatorConfig, service)
	if err != nil {
		return err
	}

	return nil
}

func deleteBase(fs afero.Fs, dir string, service string) error {
	baseServicePath := filepath.Join(dir, "base", service)
	if _, err := fs.Stat(baseServicePath); err == nil {
		err := RemoveDir(fs, baseServicePath)
		if err != nil {
			return fmt.Errorf("Could not delete "+baseServicePath, err)
		}
	}
	return nil
}

func deleteStages(fs afero.Fs, dir string, service string) error {
	stages, _ := afero.ReadDir(fs, filepath.Join(dir, "stages"))
	for _, stage := range stages {
		if stage.IsDir() {
			stageServicePath := filepath.Join(dir, "stages", stage.Name(), service)
			if _, err := fs.Stat(stageServicePath); err == nil {
				err := RemoveDir(fs, stageServicePath)
				if err != nil {
					return fmt.Errorf("Could not delete "+stageServicePath, err)
				}
			}
		}
	}
	return nil
}

func deleteServiceFromOperatorConfig(fs afero.Fs, dir string, operatorConfig KeptnConfig, service string) error {
	for index, operatorService := range operatorConfig.Services {
		if operatorService.Name == service {
			operatorConfig.Services = append(operatorConfig.Services[:index], operatorConfig.Services[index+1:]...)
			operatorBytes, err := yaml.Marshal(operatorConfig)
			if err != nil {
				fmt.Println("Could not marshal operator config", err)
			}
			err = afero.WriteFile(fs, filepath.Join(dir, ".keptn", "config.yaml"), operatorBytes, 0644)
			if err != nil {
				return fmt.Errorf("Could not write config file in "+dir+"/.keptn/config.yaml", err)
			}
			break
		}
	}
	return nil
}

func getStageAndSequence(fs afero.Fs, dir string) (stage string, sequence string, err error) {
	shipyardConfig, err := readShipyardConfigFromFile(fs, dir)
	if err != nil {
		return stage, sequence, err
	}

	if *triggerDeployParams.Stage != "" {
		foundStage := false
		for _, specStage := range shipyardConfig.Spec.Stages {
			if specStage.Name == *triggerDeployParams.Stage {
				stage = *triggerDeployParams.Stage
				foundStage = true
				break
			}
		}
		if !foundStage {
			return stage, sequence, fmt.Errorf("Could not find stage '%v' in shipyard.yaml", *triggerDeployParams.Stage)
		}
	} else if len(shipyardConfig.Spec.Stages) >= 1 {
		stage = shipyardConfig.Spec.Stages[0].Name
	} else {
		return stage, sequence, fmt.Errorf("No stage defined in shipyard.yaml")
	}

	if *triggerDeployParams.Sequence != "" {
		foundSequence := false
		for _, specStage := range shipyardConfig.Spec.Stages {
			if specStage.Name == stage {
				for _, specSequence := range specStage.Sequences {
					if specSequence.Name == *triggerDeployParams.Sequence {
						sequence = *triggerDeployParams.Sequence
						foundSequence = true
						break
					}
				}
			}
		}
		if !foundSequence {
			return stage, sequence, fmt.Errorf("Could not find sequence '%v' in stage '%v' in shipyard.yaml", *triggerDeployParams.Sequence, stage)
		}
	} else if len(shipyardConfig.Spec.Stages[0].Sequences) >= 1 {
		sequence = shipyardConfig.Spec.Stages[0].Sequences[0].Name
	} else {
		return stage, sequence, fmt.Errorf("No sequence defined in stage '%v' in shipyard.yaml", stage)
	}

	return
}
