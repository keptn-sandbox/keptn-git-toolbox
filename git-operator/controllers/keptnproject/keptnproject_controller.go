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

package keptnproject

import (
	"context"
	"encoding/json"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/go-logr/logr"
	"gopkg.in/yaml.v3"
	"io/ioutil"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"github.com/keptn-sandbox/keptn-git-toolbox/git-operator/model"
	"os"
	"path/filepath"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	keptnv1 "github.com/keptn-sandbox/keptn-git-toolbox/git-operator/api/v1"
)

// KeptnProjectReconciler reconciles a KeptnProject object
type KeptnProjectReconciler struct {
	client.Client
	Scheme           *runtime.Scheme
	ReqLogger        logr.Logger
	KeptnCredentials model.GitCredentials
}

//+kubebuilder:rbac:groups=keptn.operator.keptn.sh,resources=keptnprojects,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=keptn.operator.keptn.sh,resources=keptnprojects/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=keptn.operator.keptn.sh,resources=keptnprojects/finalizers,verbs=update

func (r *KeptnProjectReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.ReqLogger = ctrl.Log.WithValues("Request.Namespace", req.Namespace, "Request.Name", req.Name)
	r.ReqLogger.Info("Reconciling KeptnProject")

	project := &keptnv1.KeptnProject{}
	err := r.Client.Get(ctx, req.NamespacedName, project)
	if errors.IsNotFound(err) {
		r.ReqLogger.Info("KeptnProject resource not found. Ignoring since object must be deleted")
		return ctrl.Result{}, nil
	}

	secret := &corev1.Secret{}
	err = r.Client.Get(ctx, types.NamespacedName{Name: "git-credentials-" + project.Name, Namespace: req.Namespace}, secret)
	if err != nil {
		r.ReqLogger.Error(err, "Could not get secret for project "+project.Name)
		return ctrl.Result{RequeueAfter: 30 * time.Second}, err
	}

	err = json.Unmarshal(secret.Data["git-credentials"], &r.KeptnCredentials)
	if err != nil {
		r.ReqLogger.Error(err, "Could not unmarshal credentials for project "+project.Name)
		return ctrl.Result{RequeueAfter: 30 * time.Second}, err
	}

	mainHead, err := r.getCommitHash("")
	if err != nil {
		return ctrl.Result{RequeueAfter: 30 * time.Second}, err
	}

	config := &model.KeptnConfig{}

	// Save new git hashes, if changed

	// GET Configuration
	dir, _ := ioutil.TempDir("", "temp_dir")

	_, err = git.PlainClone(dir, false, &git.CloneOptions{
		URL: r.KeptnCredentials.RemoteURI,
		Auth: &http.BasicAuth{
			Username: r.KeptnCredentials.User,
			Password: r.KeptnCredentials.Token,
		},
		SingleBranch: true,
	})
	if err != nil {
		r.ReqLogger.Error(err, "Could not checkout "+project.Name)
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	if _, err := os.Stat(filepath.Join(dir, ".keptn/config.yaml")); err == nil {
		yamlFile, err := ioutil.ReadFile(filepath.Join(dir, ".keptn/config.yaml"))
		if err != nil {
			return ctrl.Result{RequeueAfter: 30 * time.Second}, err
		}
		err = yaml.Unmarshal(yamlFile, config)
		if err != nil {
			return ctrl.Result{RequeueAfter: 30 * time.Second}, err
		}

		for _, service := range config.Services {
			err = r.createKeptnService(ctx, project, service, req.Namespace)
			if err != nil {
				r.ReqLogger.Error(err, "Could not create service "+project.Name+"/"+service.Name)
				return ctrl.Result{RequeueAfter: 30 * time.Second}, err
			}
		}
	} else {
		r.ReqLogger.Info("There is no configuration file for project " + project.Name)
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	defer os.RemoveAll(dir)

	for _, service := range r.getKeptnServices(ctx, project.Name).Items {
		found := false
		for _, configService := range config.Services {
			if service.Spec.Project == project.Name && service.Spec.Service == configService.Name {
				found = true
			}
		}
		if !found {
			err = r.removeService(ctx, project.Name, service.Spec.Service, req.Namespace)
			if err != nil {
				r.ReqLogger.Error(err, "Could not remove Service "+service.Spec.Service)
				return ctrl.Result{RequeueAfter: 30 * time.Second}, err
			}
			return ctrl.Result{}, nil
		}
	}

	if project.Status.LastMainCommit != mainHead {
		for _, service := range config.Services {
			err = r.triggerDeployment(ctx, project.Name, service, config.Metadata.InitBranch, req.Namespace)
			if err != nil {
				r.ReqLogger.Error(err, "Could not trigger deployment "+service.Name)
				return ctrl.Result{RequeueAfter: 30 * time.Second}, err
			}
		}
	}
	project.Status.LastMainCommit = mainHead

	err = r.Client.Update(ctx, project)
	if err != nil {
		r.ReqLogger.Error(err, "Could not update LastAppCommit")
		return ctrl.Result{RequeueAfter: 30 * time.Second}, err
	}

	r.ReqLogger.Info("Finished Reconciling")

	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *KeptnProjectReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&keptnv1.KeptnProject{}).
		Complete(r)
}

func (r *KeptnProjectReconciler) createKeptnService(ctx context.Context, project *keptnv1.KeptnProject, service model.KeptnService, namespace string) error {
	currentKService := keptnv1.KeptnService{}
	kService := keptnv1.KeptnService{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      project.Name + "-" + service.Name,
			Labels: map[string]string{
				"project": project.Name,
			},
		},
		Spec: keptnv1.KeptnServiceSpec{
			Project: project.Name,
			Service: service.Name,
		},
		Status: keptnv1.KeptnServiceStatus{
			CreationPending: true,
		},
	}

	if err := controllerutil.SetControllerReference(project, &kService, r.Scheme); err != nil {
		r.ReqLogger.Error(err, "Failed setting Controller Reference for Service"+service.Name)
		return err
	}

	if err := r.Client.Get(ctx, types.NamespacedName{Name: project.Name + "-" + service.Name, Namespace: namespace}, &currentKService); err != nil && errors.IsNotFound(err) {
		r.ReqLogger.Info("Creating a new " + service.Name + " Service")
		err = r.Client.Create(ctx, &kService)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *KeptnProjectReconciler) triggerDeployment(ctx context.Context, project string, service model.KeptnService, initBranch string, namespace string) error {

	keptnService := keptnv1.KeptnService{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: project + "-" + service.Name, Namespace: namespace}, &keptnService)
	if err != nil {
		r.ReqLogger.Info("Could not fetch KeptnService " + project + "/" + service.Name)
	}

	newVersion, author, commitHash := r.getServiceVersion(service)
	if newVersion != keptnService.Status.DesiredVersion {
		stage := initBranch
		if stage == "" {
			stage = service.Stage
		}

		keptnService.Status.DesiredVersion = newVersion
		keptnService.Status.LastAuthor = author
		keptnService.Status.LastSourceCommitHash = commitHash
		keptnService.Spec.StartStage = stage
		keptnService.Spec.TriggerCommand = service.DeploymentTrigger
		keptnService.Status.DeploymentPending = true
		err = r.Client.Update(ctx, &keptnService)
		if err != nil {
			r.ReqLogger.Error(err, "Could not update KeptnService "+service.Name)
			return err
		} else {
			r.ReqLogger.Info("Updated Service")
		}
	}

	return nil
}

func (r *KeptnProjectReconciler) removeService(ctx context.Context, project string, service string, namespace string) error {

	keptnService := keptnv1.KeptnService{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: project + "-" + service, Namespace: namespace}, &keptnService)
	if err != nil {
		r.ReqLogger.Info("Could not fetch KeptnService " + project + "/" + service)
	}

	if keptnService.Status.SafeToDelete {
		err = r.Client.Delete(ctx, &keptnService)
		if err != nil {
			r.ReqLogger.Error(err, "Deletion of "+keptnService.Name+" was unsuccessful")
			return err
		} else {
			r.ReqLogger.Info("Deletion of " + keptnService.Name + " was successful")
			return nil
		}
	}

	keptnService.Status.DeletionPending = true
	err = r.Client.Update(ctx, &keptnService)
	if err != nil {
		r.ReqLogger.Error(err, "Could not update KeptnService "+keptnService.Name)
		return err
	} else {
		r.ReqLogger.Info("Updated Service " + keptnService.Name)
	}
	return nil
}

func (r *KeptnProjectReconciler) getCommitHash(branch string) (string, error) {

	authentication := &http.BasicAuth{
		Username: r.KeptnCredentials.User,
		Password: r.KeptnCredentials.Token,
	}

	cloneOptions := git.CloneOptions{
		URL:  r.KeptnCredentials.RemoteURI,
		Auth: authentication,
	}

	if branch != "" {
		cloneOptions = git.CloneOptions{
			URL:           r.KeptnCredentials.RemoteURI,
			Auth:          authentication,
			ReferenceName: plumbing.ReferenceName("refs/heads/" + branch),
		}
	}

	repo, err := git.Clone(memory.NewStorage(), nil, &cloneOptions)
	if err != nil {
		r.ReqLogger.Error(err, "Could not clone repository "+r.KeptnCredentials.RemoteURI)
		return "", err
	}

	head, err := repo.Head()
	if err != nil {
		r.ReqLogger.Error(err, "Could get head for "+r.KeptnCredentials.RemoteURI)
		return "", err
	}
	return head.Hash().String(), nil
}

func (r *KeptnProjectReconciler) getServiceVersion(service model.KeptnService) (version string, author string, commitHash string) {

	config := &model.DeploymentConfig{}
	authentication := &http.BasicAuth{
		Username: r.KeptnCredentials.User,
		Password: r.KeptnCredentials.Token,
	}

	cloneOptions := git.CloneOptions{
		URL:          r.KeptnCredentials.RemoteURI,
		Auth:         authentication,
		SingleBranch: true,
	}

	dir, _ := ioutil.TempDir("", "temp_dir")

	_, err := git.PlainClone(dir, false, &cloneOptions)
	if err != nil {
		r.ReqLogger.Error(err, "Could not checkout "+r.KeptnCredentials.RemoteURI)
		return "", "", ""
	}

	if _, err := os.Stat(filepath.Join(dir, "base", service.Name, "metadata/deployment.yaml")); err == nil {
		yamlFile, err := ioutil.ReadFile(filepath.Join(dir, "base", service.Name, "metadata/deployment.yaml"))
		if err != nil {
			return "", "", ""
		}

		err = yaml.Unmarshal(yamlFile, config)
		if err != nil {
			return "", "", ""
		}

	} else {
		r.ReqLogger.Info("There is no version information file for service " + service.Name)
	}
	defer os.RemoveAll(dir)
	return config.Metadata.ImageVersion, config.Metadata.Author, config.Metadata.SourceCommitHash
}

func (r *KeptnProjectReconciler) getKeptnServices(ctx context.Context, project string) keptnv1.KeptnServiceList {
	var keptnServiceList keptnv1.KeptnServiceList

	listOpts := []client.ListOption{
		client.MatchingLabels{"project": project},
	}

	err := r.Client.List(ctx, &keptnServiceList, listOpts...)
	if err != nil {
		r.ReqLogger.Error(err, "Could not get keptn services")
		return keptnServiceList
	}
	return keptnServiceList
}
