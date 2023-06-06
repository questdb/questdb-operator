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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	volumesnapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	crdv1beta1 "github.com/questdb/questdb-operator/api/v1beta1"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
)

// SnapshotReconciler reconciles a Snapshot object
type SnapshotReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=crd.questdb.io,resources=snapshots,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=crd.questdb.io,resources=snapshots/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=crd.questdb.io,resources=snapshots/finalizers,verbs=update
//+kubebuilder:rbac:groups=snapshot.storage.k8s.io,resources=volumesnapshots,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *SnapshotReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	// todo: Reconcile the VolumeSnapshot objects

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SnapshotReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&crdv1beta1.Snapshot{}).
		Owns(&volumesnapshotv1.VolumeSnapshot{}).
		Owns(&batchv1.Job{}).
		Complete(r)
}

func buildPreSnapshotJob(snap *crdv1beta1.Snapshot) batchv1.Job {
	return buildGenericSnapshotJob(snap, "pre-snapshot", "SNAPSHOT PREPARE;")
}

func buildPostSnapshotJob(snap *crdv1beta1.Snapshot) batchv1.Job {
	return buildGenericSnapshotJob(snap, "post-snapshot", "SNAPSHOT COMPLETE;")
}

func buildGenericSnapshotJob(snap *crdv1beta1.Snapshot, nameSuffix, command string) batchv1.Job {
	return batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:        fmt.Sprintf("%s-%s", snap.Name, nameSuffix),
			Namespace:   snap.Namespace,
			Labels:      snap.Labels,
			Annotations: snap.Annotations,
		},
		Spec: batchv1.JobSpec{
			Completions: pointer.Int32(1),
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:        fmt.Sprintf("%s-%s", snap.Name, nameSuffix),
					Namespace:   snap.Namespace,
					Labels:      snap.Labels,
					Annotations: snap.Annotations,
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:  "psql",
							Image: "postgres:13.3", // todo: Make this variable
							Command: []string{
								"psql",
								"-c",
								command,
							},
							Env: []v1.EnvVar{
								{
									Name:  "PGHOST",
									Value: "localhost", // todo: use questdb service name
								},
								{
									Name:  "PGUSER",
									Value: "postgres", // todo: use secret (or mount?)
								},
								{
									Name:  "PGPASSWORD",
									Value: "postgres", // todo: use secret (or mount?)
								},
								{
									Name:  "PGDATABASE",
									Value: "qdb",
								},
								{
									Name:  "PGPORT",
									Value: "5432", // todo: use questdb psql service port
								},
							},
						},
					},
					RestartPolicy: v1.RestartPolicyNever,
				},
			},
		},
	}
}

func buildVolumeSnapshot(snap *crdv1beta1.Snapshot) volumesnapshotv1.VolumeSnapshot {
	return volumesnapshotv1.VolumeSnapshot{
		ObjectMeta: snap.ObjectMeta,
		Spec:       snap.Spec.Snapshot,
	}
}
