/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"
	"github.com/fluxcd/pkg/untar"
	"github.com/go-logr/logr"
	keptnutils "github.com/keptn/go-utils/pkg/lib/v0_2_0"
	"gopkg.in/yaml.v3"
	"io/ioutil"
	"k8s.io/apimachinery/pkg/runtime"
	"net/http"
	"os"
	"path/filepath"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"

	sourcev1 "github.com/fluxcd/source-controller/api/v1beta1"
)

// KeptnGitOpsRepositoryReconciler reconciles a KeptnGitOpsRepository object
type KeptnGitOpsRepositoryReconciler struct {
	client.Client
	Scheme    *runtime.Scheme
	ReqLogger logr.Logger
}

//+kubebuilder:rbac:groups=operator.keptn.sh,resources=keptngitopsrepositories,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=operator.keptn.sh,resources=keptngitopsrepositories/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=operator.keptn.sh,resources=keptngitopsrepositories/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the KeptnGitOpsRepository object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.10.0/pkg/reconcile
func (r *KeptnGitOpsRepositoryReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.ReqLogger = ctrl.Log.WithValues("Request.Namespace", req.Namespace, "Request.Name", req.Name)
	r.ReqLogger.Info("Reconciling GitRepository")

	// get source object
	var repository sourcev1.GitRepository
	if err := r.Get(ctx, req.NamespacedName, &repository); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	r.ReqLogger.Info("New revision detected", "revision", repository.Status.Artifact.Revision)

	// create tmp dir
	tmpDir, err := ioutil.TempDir("", repository.Name)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to create temp dir, error: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	_, err = r.fetchArtifact(ctx, repository, tmpDir)
	if err != nil {
		r.ReqLogger.Error(err, "unable to get structure")
		return ctrl.Result{}, err
	}

	config, err := r.getDeploymentConfig(tmpDir)
	if err != nil {
		r.ReqLogger.Error(err, "unable to deployment configuration")
		return ctrl.Result{}, err
	}

	deploymentMap, err := getEnvironmentMap(config)
	if err != nil {
		r.ReqLogger.Error(err, "unable to create environment map")
		return ctrl.Result{}, err
	}

	_, shipyard, err := r.createShipyard(deploymentMap)
	fmt.Println(string(shipyard))

	r.ReqLogger.Info("Finished Reconciling GitRepository")

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *KeptnGitOpsRepositoryReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&sourcev1.GitRepository{}, builder.WithPredicates(GitRepositoryRevisionChangePredicate{})).
		Complete(r)
}

func (r *KeptnGitOpsRepositoryReconciler) fetchArtifact(ctx context.Context, repository sourcev1.GitRepository, dir string) (string, error) {
	if repository.Status.Artifact == nil {
		return "", fmt.Errorf("respository %s does not containt an artifact", repository.Name)
	}

	url := repository.Status.Artifact.URL

	if hostname := os.Getenv("SOURCE_HOST"); hostname != "" {
		url = fmt.Sprintf("http://%s/gitrepository/%s/%s/latest.tar.gz", hostname, repository.Namespace, repository.Name)
	}

	// download the tarball
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create HTTP request, error: %w", err)
	}

	resp, err := http.DefaultClient.Do(req.WithContext(ctx))
	if err != nil {
		return "", fmt.Errorf("failed to download artifact from %s, error: %w", url, err)
	}
	defer resp.Body.Close()

	// check response
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to download artifact, status: %s", resp.Status)
	}

	// extract
	summary, err := untar.Untar(resp.Body, dir)
	if err != nil {
		return "", fmt.Errorf("faild to untar artifact, error: %w", err)
	}

	return summary, nil
}

func (r *KeptnGitOpsRepositoryReconciler) getDeploymentConfig(dir string) (KeptnGitOpsStructure, error) {
	config := &KeptnGitOpsStructure{}
	if _, err := os.Stat(filepath.Join(dir, "config.yaml")); err == nil {
		yamlFile, err := ioutil.ReadFile(filepath.Join(dir, "config.yaml"))
		if err != nil {
			return KeptnGitOpsStructure{}, err
		}
		err = yaml.Unmarshal(yamlFile, config)

		if err != nil {
			return KeptnGitOpsStructure{}, err
		}
		return *config, nil
	} else {
		r.ReqLogger.Info("There is no configuration file")
		return KeptnGitOpsStructure{}, err
	}
}

func (r *KeptnGitOpsRepositoryReconciler) createShipyard(envMap []EnvironmentMap) (shipyard keptnutils.Shipyard, shipyardYaml []byte, error error) {
	shipyard = keptnutils.Shipyard{}

	for _, environment := range envMap {
		stage := keptnutils.Stage{
			Name:      environment.Environment,
			Sequences: environment.Sequences,
		}
		shipyard.Spec.Stages = append(shipyard.Spec.Stages, stage)
	}

	shipyardYaml, error = yaml.Marshal(shipyard)

	return
}

func getEnvironmentMap(config KeptnGitOpsStructure) ([]EnvironmentMap, error) {
	envMap := []EnvironmentMap{}

	if config.Stages != nil {
		for _, stage := range config.Stages {
			fmt.Println("Processing Stage: " + stage.Name)
			if len(stage.Environments) > 0 {
				for _, environment := range stage.Environments {
					envMap = append(envMap, EnvironmentMap{Stage: stage.Name, Environment: environment, Sequences: stage.Sequences})
				}
			}
		}
	}
	return envMap, nil
}
