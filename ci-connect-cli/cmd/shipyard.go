package cmd

import (
	"fmt"
	"github.com/spf13/afero"
	"gopkg.in/yaml.v3"
	"path/filepath"
)

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