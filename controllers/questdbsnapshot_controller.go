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
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	volumesnapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	crdv1beta1 "github.com/questdb/questdb-operator/api/v1beta1"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// QuestDBSnapshotReconciler reconciles a QuestDBSnapshot object
type QuestDBSnapshotReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

//+kubebuilder:rbac:groups=crd.questdb.io,resources=questdbsnapshots,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=crd.questdb.io,resources=questdbsnapshots/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=crd.questdb.io,resources=questdbsnapshots/finalizers,verbs=update
//+kubebuilder:rbac:groups=questdbsnapshot.storage.k8s.io,resources=volumesnapshots,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *QuestDBSnapshotReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var (
		err error

		_    = log.FromContext(ctx)
		snap = &crdv1beta1.QuestDBSnapshot{}
	)

	// Try to get the object we are reconciling.  Exit if it does not exist
	if err = r.Get(ctx, req.NamespacedName, snap); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Handle finalizer.  This will ensure that the "SNAPSHOT COMPLETE;" is run before the snapshot is deleted
	if err = r.handleFinalizer(snap); err != nil {
		return ctrl.Result{}, err
	}

	switch snap.Status.Phase {
	case "":
		// Set the phase to pending
		snap.Status.Phase = crdv1beta1.SnapshotPending
		if err = r.Status().Update(ctx, snap); err != nil {
			return ctrl.Result{}, err
		}
		r.Recorder.Eventf(snap, v1.EventTypeNormal, "SnapshotPending", "Snapshot %s is pending", snap.Name)
		return ctrl.Result{}, nil
	case crdv1beta1.SnapshotPending:
		// Create the pre-snapshot job
		job, err := r.buildPreSnapshotJob(snap)
		if err != nil {
			return ctrl.Result{}, err
		}
		if err = r.Create(ctx, &job); err != nil {
			if apierrors.IsAlreadyExists(err) {
				err = r.Get(ctx, client.ObjectKey{Name: job.Name, Namespace: job.Namespace}, &job)
			}
			if err != nil {
				return ctrl.Result{}, err
			}
		}

		// Check if the pre-snapshot job is complete
		if job.Status.Succeeded == 1 {
			// Set the phase to running
			snap.Status.Phase = crdv1beta1.SnapshotRunning
			if err = r.Status().Update(ctx, snap); err != nil {
				return ctrl.Result{}, err
			}
			r.Recorder.Eventf(snap, v1.EventTypeNormal, "SnapshotRunning", "Snapshot %s is running", snap.Name)
			return ctrl.Result{}, nil
		}

	case crdv1beta1.SnapshotRunning:
		// Create the volume snapshot
		volumeSnap, err := r.buildVolumeSnapshot(snap)
		if err != nil {
			return ctrl.Result{}, err
		}
		if err = r.Create(ctx, &volumeSnap); err != nil {
			if apierrors.IsAlreadyExists(err) {
				err = r.Get(ctx, client.ObjectKey{Name: volumeSnap.Name, Namespace: volumeSnap.Namespace}, &volumeSnap)
			}
			if err != nil {
				return ctrl.Result{}, err
			}
		}

		// Check if the snapshot job is complete
		if volumeSnap.Status != nil && volumeSnap.Status.ReadyToUse != nil && *volumeSnap.Status.ReadyToUse {
			// Set the phase to finalizing
			snap.Status.Phase = crdv1beta1.SnapshotFinalizing
			if err = r.Status().Update(ctx, snap); err != nil {
				return ctrl.Result{}, err
			}
			r.Recorder.Eventf(snap, v1.EventTypeNormal, "SnapshotFinalizing", "Snapshot %s is cleaning", snap.Name)
			return ctrl.Result{}, nil
		}

	case crdv1beta1.SnapshotFinalizing:
		// Create the pre-snapshot job
		job, err := r.buildPostSnapshotJob(snap)
		if err != nil {
			return ctrl.Result{}, err
		}
		if err = r.Create(ctx, &job); err != nil {
			if apierrors.IsAlreadyExists(err) {
				err = r.Get(ctx, client.ObjectKey{Name: job.Name, Namespace: job.Namespace}, &job)
			}
			if err != nil {
				return ctrl.Result{}, err
			}
		}

		// Check if the post-snapshot job is complete
		if job.Status.Succeeded == 1 {
			// Set the phase to running
			snap.Status.Phase = crdv1beta1.SnapshotSucceeded
			if err = r.Status().Update(ctx, snap); err != nil {
				return ctrl.Result{}, err
			}
			r.Recorder.Eventf(snap, v1.EventTypeNormal, "SnapshotSucceeded", "Snapshot %s succeeded", snap.Name)
			return ctrl.Result{}, nil
		}
	case crdv1beta1.SnapshotFailed:
		// todo: figure out what to do if the snapshot failed .. need to run snapshot complete
		return ctrl.Result{}, nil
		// r.Recorder.Eventf(snap, v1.EventTypeWarning, "SnapshotFailed", "Snapshot %s failed", snap.Name)
	case crdv1beta1.SnapshotSucceeded:
		// todo: figure out what to do if the snapshot succeeded
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *QuestDBSnapshotReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&crdv1beta1.QuestDBSnapshot{}).
		Owns(&volumesnapshotv1.VolumeSnapshot{}).
		Owns(&batchv1.Job{}).
		Complete(r)
}

func (r *QuestDBSnapshotReconciler) buildPreSnapshotJob(snap *crdv1beta1.QuestDBSnapshot) (batchv1.Job, error) {
	return r.buildGenericSnapshotJob(snap, "pre-snapshot", "SNAPSHOT PREPARE;")
}

func (r *QuestDBSnapshotReconciler) buildPostSnapshotJob(snap *crdv1beta1.QuestDBSnapshot) (batchv1.Job, error) {
	return r.buildGenericSnapshotJob(snap, "post-snapshot", "SNAPSHOT COMPLETE;")
}

func (r *QuestDBSnapshotReconciler) buildGenericSnapshotJob(snap *crdv1beta1.QuestDBSnapshot, nameSuffix, command string) (batchv1.Job, error) {
	var err error
	job := batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", snap.Name, nameSuffix),
			Namespace: snap.Namespace,
			Labels:    snap.Labels,
		},
		Spec: batchv1.JobSpec{
			Completions: pointer.Int32(1),
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("%s-%s", snap.Name, nameSuffix),
					Namespace: snap.Namespace,
					Labels:    snap.Labels,
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:  "psql",
							Image: "postgres:13.3", // todo: Make this variable.. probably in env var or setup config
							Command: []string{
								"psql",
								"-c",
								command,
							},
							Env: []v1.EnvVar{
								{
									Name:  "PGHOST",
									Value: fmt.Sprintf("%s.%s.svc.cluster.local", snap.Spec.QuestDB, snap.Namespace),
								},
								{
									Name:  "PGUSER",
									Value: "admin", // todo: use secret (or mount?)
								},
								{
									Name:  "PGPASSWORD",
									Value: "quest", // todo: use secret (or mount?)
								},
								{
									Name:  "PGDATABASE",
									Value: "qdb",
								},
								{
									Name:  "PGPORT",
									Value: "8812", // todo: make this variable (based on service port)
								},
							},
						},
					},
					RestartPolicy: v1.RestartPolicyNever,
				},
			},
		},
	}

	err = ctrl.SetControllerReference(snap, &job, r.Scheme)
	return job, err
}

func (r *QuestDBSnapshotReconciler) buildVolumeSnapshot(snap *crdv1beta1.QuestDBSnapshot) (volumesnapshotv1.VolumeSnapshot, error) {
	var err error
	volSnap := volumesnapshotv1.VolumeSnapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name:      snap.Name,
			Namespace: snap.Namespace,
			Labels:    snap.Labels,
		},
		Spec: volumesnapshotv1.VolumeSnapshotSpec{
			Source: volumesnapshotv1.VolumeSnapshotSource{
				PersistentVolumeClaimName: pointer.String(snap.Spec.QuestDB),
			},
		},
	}

	if snap.Spec.VolumeSnapshotClass != "" {
		volSnap.Spec.VolumeSnapshotClassName = pointer.String(snap.Spec.VolumeSnapshotClass)
	}

	// todo: figure out why we aren't waiting for the volumesnapshot to delete before cleaning up the snapshot
	err = ctrl.SetControllerReference(snap, &volSnap, r.Scheme)
	return volSnap, err

}

const (
	snapshotFinalizer = "questdbsnapshot.crd.questdb.io/finalizer"
)

func (r *QuestDBSnapshotReconciler) handleFinalizer(snap *crdv1beta1.QuestDBSnapshot) error {
	if snap.DeletionTimestamp.IsZero() {
		if !controllerutil.ContainsFinalizer(snap, snapshotFinalizer) {
			controllerutil.AddFinalizer(snap, snapshotFinalizer)
			if err := r.Update(context.Background(), snap); err != nil {
				return err
			}
		}
	} else {
		if controllerutil.ContainsFinalizer(snap, snapshotFinalizer) {
			// Wait for the snapshot phase to be either failed or succeeded before deleting the snapshot
			if snap.Status.Phase == crdv1beta1.SnapshotFailed || snap.Status.Phase == crdv1beta1.SnapshotSucceeded {
				controllerutil.RemoveFinalizer(snap, snapshotFinalizer)
				if err := r.Update(context.Background(), snap); err != nil {
					return err
				}
			}
		}
	}

	return nil
}
