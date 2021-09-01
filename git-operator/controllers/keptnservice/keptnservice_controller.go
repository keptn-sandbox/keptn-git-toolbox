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

package keptnservice

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"github.com/keptn-sandbox/keptn-git-toolbox/git-operator/model"
	nethttp "net/http"
	"os"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	keptnv1 "github.com/keptn-sandbox/keptn-git-toolbox/git-operator/api/v1"
)

// KeptnServiceReconciler reconciles a KeptnService object
type KeptnServiceReconciler struct {
	client.Client
	Scheme    *runtime.Scheme
	ReqLogger logr.Logger
	keptnApi  string
}

//+kubebuilder:rbac:groups=keptn.operator.keptn.sh,resources=keptnservices,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=keptn.operator.keptn.sh,resources=keptnservices/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=keptn.operator.keptn.sh,resources=keptnservices/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the KeptnService object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.8.3/pkg/reconcile
func (r *KeptnServiceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.ReqLogger = ctrl.Log.WithValues("Request.Namespace", req.Namespace, "Request.Name", req.Name)
	r.ReqLogger.Info("Reconciling KeptnService")

	var ok bool
	r.keptnApi, ok = os.LookupEnv("KEPTN_API_ENDPOINT")
	if !ok {
		r.ReqLogger.Info("KEPTN_API_ENDPOINT is not present, defaulting to api-gateway-nginx")
		r.keptnApi = "http://api-gateway-nginx/api"
	}

	service := &keptnv1.KeptnService{}
	err := r.Client.Get(ctx, req.NamespacedName, service)
	if errors.IsNotFound(err) {
		r.ReqLogger.Info("KeptnProject resource not found. Ignoring since object must be deleted")
		return ctrl.Result{RequeueAfter: 30 * time.Second}, err
	}

	if service.Status.CreationPending && !r.checkKeptnServiceExists(ctx, service, req.Namespace) {
		service.Status.LastSetupStatus, err = r.createService(ctx, service.Spec.Service, req.Namespace, service.Spec.Project)
		if err != nil {
			r.ReqLogger.Error(err, "Could not create service "+service.Spec.Service)
			return ctrl.Result{RequeueAfter: 30 * time.Second}, err
		}
		service.Status.CreationPending = false
	}

	if service.Status.DeploymentPending {
		r.ReqLogger.Info("Deployment is pending")
		err = r.triggerDeployment(ctx, service.Spec.Service, req.Namespace, service.Spec.Project, service.Spec.StartStage, service.Spec.TriggerCommand, service.Status.DesiredVersion, service.Status.LastAuthor, service.Status.LastSourceCommitHash)
		if err != nil {
			return ctrl.Result{RequeueAfter: 60 * time.Second}, err
		}
		service.Status.DeploymentPending = false
		err = r.Client.Update(ctx, service)
		if err != nil {
			r.ReqLogger.Error(err, "Could not update Service")
			return ctrl.Result{RequeueAfter: 30 * time.Second}, err
		}
		return ctrl.Result{}, nil
	}

	if service.Status.DeletionPending && !service.Status.SafeToDelete {
		r.ReqLogger.Info("Deletion is pending")
		err = r.deleteService(ctx, service.Spec.Service, req.Namespace, service.Spec.Project)
		if err != nil {
			r.ReqLogger.Error(err, "Could not delete Service")
			return ctrl.Result{RequeueAfter: 60 * time.Second}, err
		}
		service.Status.SafeToDelete = true
	}

	err = r.Client.Update(ctx, service)
	if err != nil {
		r.ReqLogger.Error(err, "Could not update Service")
		return ctrl.Result{RequeueAfter: 30 * time.Second}, err
	}

	r.ReqLogger.Info("Finished Reconciling")

	return ctrl.Result{RequeueAfter: 180 * time.Second}, nil

}

// SetupWithManager sets up the controller with the Manager.
func (r *KeptnServiceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&keptnv1.KeptnService{}).
		Complete(r)
}

func (r *KeptnServiceReconciler) createService(ctx context.Context, service string, namespace string, project string) (int, error) {
	httpclient := nethttp.Client{
		Timeout: 30 * time.Second,
	}

	data, _ := json.Marshal(map[string]string{
		"serviceName": service,
	})

	keptnToken := r.getKeptnToken(ctx, namespace)

	request, err := nethttp.NewRequest("POST", r.keptnApi+"/controlPlane/v1/project/"+project+"/service", bytes.NewBuffer(data))
	if err != nil {
		r.ReqLogger.Error(err, "Could not create service "+service)
		return 0, err
	}

	request.Header.Set("content-type", "application/json")
	request.Header.Set("x-token", keptnToken)

	r.ReqLogger.Info("Creating Keptn Service " + service)
	response, err := httpclient.Do(request)
	if err != nil {
		return 0, err
	}
	return response.StatusCode, err
}

func (r *KeptnServiceReconciler) deleteService(ctx context.Context, service string, namespace string, project string) error {
	httpclient := nethttp.Client{
		Timeout: 30 * time.Second,
	}

	keptnToken := r.getKeptnToken(ctx, namespace)

	request, err := nethttp.NewRequest("DELETE", r.keptnApi+"/controlPlane/v1/project/"+project+"/service/"+service, bytes.NewBuffer(nil))
	if err != nil {
		r.ReqLogger.Error(err, "Could not delete service "+service)
	}

	request.Header.Set("content-type", "application/json")
	request.Header.Set("x-token", keptnToken)

	r.ReqLogger.Info("Deleting Keptn Service " + service)
	_, err = httpclient.Do(request)
	if err != nil {
		return err
	}
	return err
}

func (r *KeptnServiceReconciler) triggerDeployment(ctx context.Context, service string, namespace string, project string, stage string, trigger string, version string, author string, sourceGitHash string) error {

	httpclient := nethttp.Client{
		Timeout: 30 * time.Second,
	}

	labels := make(map[string]string)

	if version != "" {
		labels["version"] = version
		labels["buildId"] = version
	}

	if author != "" {
		labels["author"] = author
	}

	if sourceGitHash != "" {
		labels["sourceGitHash"] = sourceGitHash
	}

	data, err := json.Marshal(model.KeptnTriggerEvent{
		ContentType: "application/json",
		Data: model.KeptnEventData{
			Service: service,
			Project: project,
			Stage:   stage,
			Labels:  labels,
			Image:   service + ":" + version,
		},
		Source:      "Keptn GitOps Operator",
		SpecVersion: "1.0",
		Type:        trigger,
	})
	if err != nil {
		r.ReqLogger.Info("Could not marshal Keptn Trigger Event")
	}

	keptnToken := r.getKeptnToken(ctx, namespace)

	r.ReqLogger.Info("Triggering Deployment " + service)
	request, err := nethttp.NewRequest("POST", r.keptnApi+"/v1/event", bytes.NewBuffer(data))
	if err != nil {
		r.ReqLogger.Error(err, "Could not trigger deployment "+service)
		return err
	}

	request.Header.Set("content-type", "application/cloudevents+json")
	request.Header.Set("x-token", keptnToken)

	_, err = httpclient.Do(request)
	if err != nil {
		return err
	}

	return err
}

func (r *KeptnServiceReconciler) checkKeptnServiceExists(ctx context.Context, service *keptnv1.KeptnService, namespace string) bool {
	httpclient := nethttp.Client{
		Timeout: 30 * time.Second,
	}

	keptnToken := r.getKeptnToken(ctx, namespace)

	request, err := nethttp.NewRequest("GET", r.keptnApi+"/controlPlane/v1/project/"+service.Spec.Project+"/stage/"+service.Spec.StartStage+"/service/"+service.Spec.Service+"/resource", bytes.NewBuffer(nil))
	if err != nil {
		r.ReqLogger.Error(err, "Could not check if service exists "+service.Spec.Service)
	}

	request.Header.Set("x-token", keptnToken)

	response, err := httpclient.Do(request)
	if err != nil || response.StatusCode != 200 {
		return false
	}
	r.ReqLogger.Info("Keptn Service already exists: " + service.Name)
	return true

}

func (r *KeptnServiceReconciler) getKeptnToken(ctx context.Context, namespace string) string {
	keptnToken := &corev1.Secret{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: "keptn-api-token", Namespace: namespace}, keptnToken)
	if err != nil {
		r.ReqLogger.Info("Could not fetch KeptnToken")
	}
	return string(keptnToken.Data["keptn-api-token"])
}
