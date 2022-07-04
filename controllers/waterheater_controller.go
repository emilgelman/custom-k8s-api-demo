/*
Copyright 2022.

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
	demov1 "github.com/emilgelman/custom-k8s-api-/api/v1"
	"github.com/pkg/errors"
	kbatch "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

// WaterHeaterReconciler reconciles a WaterHeater object
type WaterHeaterReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=demo.demo.appsflyer.com,resources=waterheaters,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=demo.demo.appsflyer.com,resources=waterheaters/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=demo.demo.appsflyer.com,resources=waterheaters/finalizers,verbs=update
//+kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the WaterHeater object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.10.0/pkg/reconcile
func (r *WaterHeaterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var waterheater demov1.WaterHeater
	if err := r.Get(ctx, client.ObjectKey{Namespace: req.Namespace, Name: req.Name}, &waterheater); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	diff := waterheater.Spec.Temperature - waterheater.Status.Temperature
	if diff == 0 {
		return ctrl.Result{}, nil
	}

	if r.jobSucceeded(ctx, req.Name) {
		waterheater.Status.Temperature = waterheater.Spec.Temperature
		waterheater.Status.Mode = demov1.Idle
	} else {
		newMode := demov1.Heat
		if diff < 0 {
			newMode = demov1.Cool
			diff *= -1
		}
		waterheater.Status.Mode = newMode
		if err := r.runJob(ctx, req, waterheater, diff); err != nil {
			return ctrl.Result{}, err
		}
	}

	if err := r.Status().Update(ctx, &waterheater); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *WaterHeaterReconciler) runJob(ctx context.Context, req ctrl.Request, waterheater demov1.WaterHeater, diff int64) error {
	job := r.ConstructJob(req.Name, diff)
	if err := ctrl.SetControllerReference(&waterheater, job, r.Scheme); err != nil {
		return err
	}
	if err := r.Create(ctx, job); err != nil {
		return errors.Wrapf(err, "unable to create job")
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *WaterHeaterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&demov1.WaterHeater{}, builder.WithPredicates(WaterHeaterStatusChangePredicate{})).
		Owns(&kbatch.Job{}, builder.WithPredicates(JobStatusChangePredicate{})).
		Complete(r)
}

func (r *WaterHeaterReconciler) work(waterheater *demov1.WaterHeater, diff int64) {
	newStatus := demov1.Heat
	if diff < 0 {
		newStatus = demov1.Cool
		diff *= -1
	}
	waterheater.Status.Mode = newStatus
	time.Sleep(time.Second * time.Duration(diff)) // simulate work being done
}

func (r *WaterHeaterReconciler) jobSucceeded(ctx context.Context, owner string) bool {
	var jobs kbatch.JobList
	if err := r.List(ctx, &jobs, client.HasLabels{fmt.Sprintf("owner=%s", owner)}); err != nil {
		return false
	}
	if len(jobs.Items) > 0 {
		job := jobs.Items[0]
		if job.Status.Succeeded > 0 {
			return true
		}
	}
	return false

}
func (r *WaterHeaterReconciler) ConstructJob(owner string, diff int64) *kbatch.Job {
	return &kbatch.Job{
		ObjectMeta: v1.ObjectMeta{
			Name:      "job",
			Namespace: "default",
			Labels: map[string]string{
				"owner": owner,
			},
		},
		Spec: kbatch.JobSpec{
			TTLSecondsAfterFinished: pointer.Int32Ptr(10),
			Template: corev1.PodTemplateSpec{

				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{{
						Name:  "waterheater-job",
						Image: "busybox:latest",
						Args:  []string{"sleep", fmt.Sprintf("%v", diff)},
					},
					},
				},
			},
		},
	}
}
