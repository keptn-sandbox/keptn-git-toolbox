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

package keptngitconfiguration

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/fluxcd/pkg/apis/meta"
	sourcev1 "github.com/fluxcd/source-controller/api/v1beta1"

	//	"github.com/fluxcd/pkg/apis/meta"
	"github.com/go-logr/logr"
	keptnv1 "github.com/keptn-sandbox/keptn-git-toolbox/git-operator/api/v1"
	"github.com/keptn-sandbox/keptn-git-toolbox/git-operator/model"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"time"
)

// KeptnGitConfigurationReconciler reconciles a KeptnGitConfiguration object
type KeptnGitConfigurationReconciler struct {
	client.Client
	Scheme    *runtime.Scheme
	ReqLogger logr.Logger
	Recorder  record.EventRecorder
}

const requeueSeconds = 60 * time.Second

//+kubebuilder:rbac:groups=keptn.operator.keptn.sh,resources=keptngitconfigurations,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=keptn.operator.keptn.sh,resources=keptngitconfigurations/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=keptn.operator.keptn.sh,resources=keptngitconfigurations/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the KeptnGitConfiguration object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.10.0/pkg/reconcile
func (r *KeptnGitConfigurationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.ReqLogger = ctrl.Log.WithValues("Request.Namespace", req.Namespace, "Request.Name", req.Name)
	r.ReqLogger.Info("Reconciling KeptnGitConfiguration")

	config := &keptnv1.KeptnGitConfiguration{}
	err := r.Client.Get(ctx, req.NamespacedName, config)
	if errors.IsNotFound(err) {
		r.ReqLogger.Info("KeptnGitConfiguration resource not found. Ignoring since object must be deleted")
		return ctrl.Result{}, nil
	}

	keptnGitCredentials, err := r.GetGitSecret(ctx, req, *config)
	if err != nil {
		r.Recorder.Event(config, "Warning", "KeptnGitCredentialsNotFound", "Could not find GIT Credentials")
		return ctrl.Result{}, err
	}

	gitOpsSecret := GenerateGitControllerSecret(req, *config, keptnGitCredentials)

	err, requeue := r.CreateGitControllerSecret(ctx, req, *config, gitOpsSecret)
	if err != nil {
		r.Recorder.Event(config, "Warning", "KeptnGitOpsCredentialsNotCreated", "Could not create GitOps Credentials")
		return ctrl.Result{RequeueAfter: requeueSeconds}, err
	}
	if requeue {
		r.Recorder.Event(config, "Warning", "KeptnGitOpsCredentialsCreated", "Created GitOps Credentials")
		r.ReqLogger.Info(fmt.Sprintf("New Secret for Project %s Created, Requeueing", config.Spec.Project))
		return ctrl.Result{}, nil
	}

	err, requeue = r.CreateGitRepository(ctx, req, *config, keptnGitCredentials)
	if err != nil {
		r.Recorder.Event(config, "Warning", "KeptnGitOpsRepositoryNotCreated", "Could not create GitOps Repository")
		return ctrl.Result{RequeueAfter: requeueSeconds}, err
	}
	if requeue {
		r.Recorder.Event(config, "Warning", "KeptnGitOpsRepositoryCreated", "Created GitOps Repository")
		r.ReqLogger.Info(fmt.Sprintf("New GitOps Repository for Project %s Created, Requeueing", config.Spec.Project))
		return ctrl.Result{Requeue: true}, nil
	}

	return ctrl.Result{RequeueAfter: requeueSeconds}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *KeptnGitConfigurationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&keptnv1.KeptnGitConfiguration{}).
		Owns(&corev1.Secret{}).
		Complete(r)
}

func (r *KeptnGitConfigurationReconciler) GetGitSecret(ctx context.Context, req ctrl.Request, config keptnv1.KeptnGitConfiguration) (model.GitCredentials, error) {
	secret := &corev1.Secret{}
	keptnCredentials := model.GitCredentials{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: "git-credentials-" + config.Spec.Project, Namespace: req.Namespace}, secret)
	if err != nil {
		r.ReqLogger.Error(err, "Could not get secret for project "+config.Spec.Project)
		return model.GitCredentials{}, err
	}

	err = json.Unmarshal(secret.Data["git-credentials"], &keptnCredentials)
	if err != nil {
		r.ReqLogger.Error(err, "Could not unmarshal credentials for project "+config.Spec.Project)
		return model.GitCredentials{}, err
	}
	return keptnCredentials, nil
}

func GenerateGitControllerSecret(req ctrl.Request, config keptnv1.KeptnGitConfiguration, secret model.GitCredentials) corev1.Secret {
	return corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("keptn-gitops-%s", config.Spec.Project),
			Namespace: req.Namespace,
		},
		StringData: map[string]string{
			"username": secret.User,
			"password": secret.Token,
		},
	}
}

func (r *KeptnGitConfigurationReconciler) CreateGitControllerSecret(ctx context.Context, req ctrl.Request, config keptnv1.KeptnGitConfiguration, secret corev1.Secret) (error error, requeue bool) {
	currentSecret := corev1.Secret{}
	newSecret := secret

	if err := controllerutil.SetControllerReference(&config, &newSecret, r.Scheme); err != nil {
		r.ReqLogger.Error(err, "Failed setting Controller Reference for Secret"+config.Spec.Project)
		return err, true
	}

	if err := r.Client.Get(ctx, types.NamespacedName{Name: fmt.Sprintf("keptn-gitops-%s", config.Spec.Project), Namespace: req.Namespace}, &currentSecret); err != nil && errors.IsNotFound(err) {
		r.ReqLogger.Info(fmt.Sprintf("Creating a new GitOps Secret for Project: %s ", config.Spec.Project))
		err = r.Client.Create(ctx, &newSecret)
		if err != nil {
			return err, true
		} else {
			return nil, true
		}
	}
	return nil, false
}

func (r *KeptnGitConfigurationReconciler) CreateGitRepository(ctx context.Context, req ctrl.Request, config keptnv1.KeptnGitConfiguration, secret model.GitCredentials) (error error, requeue bool) {

	if config.Spec.WatchedBranch != "" {
		config.Spec.WatchedBranch = "master"
	}
	currentGitRepo := sourcev1.GitRepository{}
	newGitRepo := sourcev1.GitRepository{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "keptn-gitops-" + config.Spec.Project,
			Namespace: req.Namespace,
		},
		Spec: sourcev1.GitRepositorySpec{
			URL: secret.RemoteURI,
			Interval: metav1.Duration{
				Duration: 30 * time.Second},
			Reference: &sourcev1.GitRepositoryRef{
				Branch: config.Spec.WatchedBranch,
			},
			SecretRef: &meta.LocalObjectReference{
				Name: "keptn-gitops-%s" + config.Spec.Project,
			},
		},
	}

	if err := controllerutil.SetControllerReference(&config, &newGitRepo, r.Scheme); err != nil {
		r.ReqLogger.Error(err, "Failed setting Controller Reference for Git Repository"+config.Spec.Project)
		return err, true
	}

	if err := r.Client.Get(ctx, types.NamespacedName{Name: fmt.Sprintf("keptn-gitops-%s", config.Spec.Project), Namespace: req.Namespace}, &currentGitRepo); err != nil && errors.IsNotFound(err) {
		r.ReqLogger.Info(fmt.Sprintf("Creating a new GitOps Repository for Project: %s ", config.Spec.Project))
		err = r.Client.Create(ctx, &newGitRepo)
		if err != nil {
			return err, true
		} else {
			return nil, true
		}
	} else {
		fmt.Println(err)
	}
	return nil, false
}
