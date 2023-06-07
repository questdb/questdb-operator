/*
Copyright 2023.

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
	"time"

	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	crdv1beta1 "github.com/questdb/questdb-operator/api/v1beta1"

	volumesnapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SnapshotScheduleReconciler reconciles a SnapshotSchedule object
type SnapshotScheduleReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=crd.questdb.io,resources=snapshotschedules,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=crd.questdb.io,resources=snapshotschedules/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=crd.questdb.io,resources=snapshotschedules/finalizers,verbs=update
//+kubebuilder:rbac:groups=batch,resources=cronjobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *SnapshotScheduleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var (
		err error

		sched = &crdv1beta1.SnapshotSchedule{}
		_     = log.FromContext(ctx)
	)

	err = r.Get(ctx, req.NamespacedName, sched)

	if err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		} else {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SnapshotScheduleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&crdv1beta1.SnapshotSchedule{}).
		Owns(&batchv1.CronJob{}).
		Owns(&v1.ConfigMap{}).
		Complete(r)
}

func buildCronJob(sched *crdv1beta1.SnapshotSchedule) batchv1.CronJob {
	return batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      sched.Name,
			Namespace: sched.Namespace,
			Labels:    sched.Labels,
		},
		Spec: batchv1.CronJobSpec{
			Schedule: sched.Spec.Schedule,
			JobTemplate: batchv1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					Template: v1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Name:      sched.Name,
							Namespace: sched.Namespace,
							Labels:    sched.Labels,
						},
						Spec: v1.PodSpec{
							Containers: []v1.Container{
								{
									Name:  "busybox",
									Image: "busybox:1.36.1",
									Args:  []string{"echo", "hello"},
									VolumeMounts: []v1.VolumeMount{
										{
											Name:      "snapshot",
											MountPath: "/etc/snapshot",
										},
									},
								},
							},
							// todo: Add ServiceAccountName -- used by the pod to allow snapshot creation ONLY
							Volumes: []v1.Volume{
								{
									Name: "snapshot",
									VolumeSource: v1.VolumeSource{
										ConfigMap: &v1.ConfigMapVolumeSource{
											LocalObjectReference: v1.LocalObjectReference{
												Name: sched.Name,
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func buildSnapshotConfigMap(sched *crdv1beta1.SnapshotSchedule) (v1.ConfigMap, error) {

	snap := volumesnapshotv1.VolumeSnapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", sched.Name, time.Now().Format("20060102150405")),
			Namespace: sched.Namespace,
			Labels:    sched.Labels,
		},
		Spec: sched.Spec.Snapshot,
	}

	data, err := yaml.Marshal(snap)
	if err != nil {
		return v1.ConfigMap{}, err
	}

	return v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      sched.Name,
			Namespace: sched.Namespace,
			Labels:    sched.Labels,
		},
		Data: map[string]string{
			"snapshot.yaml": string(data),
		},
	}, nil
}
