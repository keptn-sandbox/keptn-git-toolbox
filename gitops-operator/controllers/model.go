package controllers

import keptnutils "github.com/keptn/go-utils/pkg/lib/v0_2_0"

type KeptnGitOpsStructure struct {
	Stages []StageConfig `yaml:"stages,omitempty"`
}

type StageConfig struct {
	keptnutils.Stage `yaml:",inline"`
	Environments     []string `yaml:"environments,omitempty"`
}

type EnvironmentMap struct {
	Environment string
	Stage       string
	Sequences   []keptnutils.Sequence
}
