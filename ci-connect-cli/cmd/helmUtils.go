package cmd

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/downloader"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/helmpath"
	"io/ioutil"
	"os"
	"path/filepath"
)

type ChartMeta struct {
	Name string `yaml:"name"`
	Version string `yaml:"version"`
	AppVersion string `yaml:"appVersion"`
}

var appMeta ChartMeta

func DependencyUpdate(chartPath string) error {
	// Garbage collect before the dependency update so that
	// anonymous files from previous runs are cleared, with
	// a safe guard time offset to not touch any files in
	// use.
	repositoryConfig := helmpath.ConfigPath("repositories.yaml")
	repositoryCache  := helmpath.CachePath("repository")

	fmt.Println(repositoryConfig)
	fmt.Println(repositoryCache)

	man := &downloader.Manager{
		Out: os.Stderr,
		ChartPath:        chartPath,
		RepositoryConfig: repositoryConfig,
		RepositoryCache: repositoryCache,
		Getters: getter.All(&cli.EnvSettings{
			RepositoryConfig: repositoryConfig,
			RepositoryCache:  repositoryCache,
		}),

	}
	return man.Update()
}

func getHelmChartVersion(chartPath string) (string, error) {
	config := filepath.Join(chartPath, "Chart.yaml")
	configFile, err := ioutil.ReadFile(config)
	if err != nil {
		return "", fmt.Errorf("Could not read file: %v", err)
	}
	err = yaml.Unmarshal(configFile, &appMeta)
	if err != nil {
		return "", fmt.Errorf("Could not unmarshal file: %v", err)
	}
	return appMeta.Version, nil
}

func getHelmChartAppVersion(chartPath string) (string, error) {
	config := filepath.Join(chartPath, "Chart.yaml")
	configFile, err := ioutil.ReadFile(config)
	if err != nil {
		return "", fmt.Errorf("Could not read file: %v", err)
	}
	err = yaml.Unmarshal(configFile, &appMeta)
	if err != nil {
		return "", fmt.Errorf("Could not unmarshal file: %v", err)
	}
	return appMeta.AppVersion, nil
}