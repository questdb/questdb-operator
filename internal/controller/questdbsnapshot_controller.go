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

package controller

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	volumesnapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	crdv1beta1 "github.com/questdb/questdb-operator/api/v1beta1"
	"github.com/questdb/questdb-operator/internal/secrets"
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
//+kubebuilder:rbac:groups=snapshot.storage.k8s.io,resources=volumesnapshotclasses,verbs=get;list;watch
//+kubebuilder:rbac:groups=snapshot.storage.k8s.io,resources=volumesnapshots,verbs=get;list;watch;create;update;patch;delete

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

	// Set default value for job backoff limit in case it is not set
	if snap.Spec.JobBackoffLimit == 0 {
		snap.Spec.JobBackoffLimit = crdv1beta1.JobBackoffLimitDefault
		err = r.Update(ctx, snap)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	switch snap.Status.Phase {
	case "":
		return r.handlePhaseEmpty(ctx, snap)
	case crdv1beta1.SnapshotPending:
		return r.handlePhasePending(ctx, snap)
	case crdv1beta1.SnapshotRunning:
		return r.handlePhaseRunning(ctx, snap)
	case crdv1beta1.SnapshotFinalizing:
		return r.handlePhaseFinalizing(ctx, snap)
	case crdv1beta1.SnapshotFailed:
		return r.handlePhaseFailed(ctx, snap)
	case crdv1beta1.SnapshotSucceeded:
		return r.handlePhaseSucceeded(ctx, snap)
	default:
		return ctrl.Result{}, fmt.Errorf("unknown phase %s", snap.Status.Phase)
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *QuestDBSnapshotReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&crdv1beta1.QuestDBSnapshot{}).
		Owns(&volumesnapshotv1.VolumeSnapshot{}).
		Owns(&batchv1.Job{}).
		Complete(r)
}

func (r *QuestDBSnapshotReconciler) buildPreSnapshotJob(ctx context.Context, snap *crdv1beta1.QuestDBSnapshot) (batchv1.Job, error) {
	return r.buildGenericSnapshotJob(ctx, snap, "pre-snapshot", "SNAPSHOT PREPARE;")
}

func (r *QuestDBSnapshotReconciler) buildPostSnapshotJob(ctx context.Context, snap *crdv1beta1.QuestDBSnapshot) (batchv1.Job, error) {
	return r.buildGenericSnapshotJob(ctx, snap, "post-snapshot", "SNAPSHOT COMPLETE;")
}

func (r *QuestDBSnapshotReconciler) buildGenericSnapshotJob(ctx context.Context, snap *crdv1beta1.QuestDBSnapshot, nameSuffix, command string) (batchv1.Job, error) {
	var (
		err      error
		user     = "admin"
		password = "quest"
	)

	// Get status of any related secrets. These will be used later to add pgwire credentials
	// to the pre- and post-snapshot jobs.
	s, err := secrets.GetSecrets(ctx, r.Client, client.ObjectKeyFromObject(snap))
	if err != nil {
		return batchv1.Job{}, err
	}

	if s.PsqlSecret != nil {
		if val, found := s.PsqlSecret.Data["QDB_PSQL_USER"]; found {
			user = string(val)
		}
		if val, found := s.PsqlSecret.Data["QDB_PSQL_PASSWORD"]; found {
			password = string(val)
		}
	}

	job := batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", snap.Name, nameSuffix),
			Namespace: snap.Namespace,
			Labels:    snap.Labels,
		},
		Spec: batchv1.JobSpec{
			Completions:  pointer.Int32(1),
			BackoffLimit: pointer.Int32(snap.Spec.JobBackoffLimit + 2), // adding a few extra attempts to ensure that we hit the reconciler to fail the snapshot
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
									Value: fmt.Sprintf("%s.%s.svc.cluster.local", snap.Spec.QuestDBName, snap.Namespace),
								},
								{
									Name:  "PGUSER",
									Value: user,
								},
								{
									Name:  "PGPASSWORD",
									Value: password,
								},
								{
									Name:  "PGDATABASE",
									Value: "qdb",
								},
								{
									Name:  "PGPORT",
									Value: "8812",
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
				PersistentVolumeClaimName: pointer.String(snap.Spec.QuestDBName),
			},
			VolumeSnapshotClassName: snap.Spec.VolumeSnapshotClassName,
		},
	}

	err = ctrl.SetControllerReference(snap, &volSnap, r.Scheme)
	return volSnap, err

}

func (r *QuestDBSnapshotReconciler) handlePhaseEmpty(ctx context.Context, snap *crdv1beta1.QuestDBSnapshot) (ctrl.Result, error) {
	// Check that the QuestDB exists
	qdb := &crdv1beta1.QuestDB{}
	namespacedName := types.NamespacedName{Namespace: snap.Namespace, Name: snap.Spec.QuestDBName}
	err := r.Get(ctx, namespacedName, qdb)
	if err != nil {
		if apierrors.IsNotFound(err) {
			snap.Status.Phase = crdv1beta1.SnapshotFailed
			err := r.Status().Update(ctx, snap)
			if err != nil {
				return ctrl.Result{}, err
			}
			r.Recorder.Eventf(snap, v1.EventTypeWarning, "SnapshotFailed", "QuestDB %q not found", namespacedName)
			return ctrl.Result{}, nil
		}
	}

	// Set the phase to pending
	snap.Status.Phase = crdv1beta1.SnapshotPending
	if err := r.Status().Update(ctx, snap); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *QuestDBSnapshotReconciler) handlePhasePending(ctx context.Context, snap *crdv1beta1.QuestDBSnapshot) (ctrl.Result, error) {

	// Check that the volume snapshot class exists
	if snap.Spec.VolumeSnapshotClassName != nil {
		volSnapClass := &volumesnapshotv1.VolumeSnapshotClass{}
		err := r.Get(ctx, client.ObjectKey{Name: *snap.Spec.VolumeSnapshotClassName}, volSnapClass)
		if err != nil {
			// If the volume snapshot class does not exist, fail the snapshot
			if apierrors.IsNotFound(err) {
				snap.Status.Phase = crdv1beta1.SnapshotFailed
				err = r.Status().Update(ctx, snap)
				r.Recorder.Eventf(snap, v1.EventTypeWarning, "SnapshotFailed", "VolumeSnapshotClass %s not found", *snap.Spec.VolumeSnapshotClassName)
			}
			return ctrl.Result{}, err
		}
	}

	// Create the pre-snapshot job
	job, err := r.buildPreSnapshotJob(ctx, snap)
	if err != nil {
		return ctrl.Result{}, err
	}
	if err = r.Create(ctx, &job); err != nil {
		if err == nil {
			r.Recorder.Eventf(snap, v1.EventTypeNormal, "SnapshotPending", "Running 'SNAPSHOT PREPARE;' for snapshot %s", snap.Name)
		}
		if apierrors.IsAlreadyExists(err) {
			err = r.Get(ctx, client.ObjectKey{Name: job.Name, Namespace: job.Namespace}, &job)
		}
		if err != nil {
			r.Recorder.Eventf(snap, v1.EventTypeWarning, "SnapshotPending", "Error running 'SNAPSHOT PREPARE' job: %s", err.Error())
			return ctrl.Result{}, err
		}

	}

	// Check if the pre-snapshot job is complete
	if job.Status.Succeeded == 1 {

		// Add the SnapshotCompleteFinalizer
		if !controllerutil.ContainsFinalizer(snap, crdv1beta1.SnapshotCompleteFinalizer) {
			controllerutil.AddFinalizer(snap, crdv1beta1.SnapshotCompleteFinalizer)
			err = r.Update(ctx, snap)
			if err != nil {
				return ctrl.Result{}, err
			}
		}

		// Set the phase to running, unless the snapshot is marked for deletion, in which case jump straight to finalizing
		snap.Status.Phase = crdv1beta1.SnapshotRunning
		if snap.DeletionTimestamp != nil {
			snap.Status.Phase = crdv1beta1.SnapshotFinalizing
		}

		if err = r.Status().Update(ctx, snap); err != nil {
			return ctrl.Result{}, err
		}

		if snap.DeletionTimestamp == nil {
			r.Recorder.Eventf(snap, v1.EventTypeNormal, "SnapshotRunning", "Snapshot %s is running", snap.Name)
		} else {
			r.Recorder.Eventf(snap, v1.EventTypeNormal, "SnapshotFinalizing", "Snapshot %s is finalizing", snap.Name)
		}

	}

	// If we've reached the maximum number of attempts, fail the snapshot
	if job.Status.Failed >= snap.Spec.JobBackoffLimit {
		snap.Status.Phase = crdv1beta1.SnapshotFailed
		err = r.Status().Update(ctx, snap)
		r.Recorder.Eventf(snap, v1.EventTypeWarning, "SnapshotFailed", "Error running 'SNAPSHOT PREPARE' in job %s", job.Name)
	}

	return ctrl.Result{}, err
}

func (r *QuestDBSnapshotReconciler) handlePhaseRunning(ctx context.Context, snap *crdv1beta1.QuestDBSnapshot) (ctrl.Result, error) {

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
	}

	return ctrl.Result{}, nil

}

func (r *QuestDBSnapshotReconciler) handlePhaseFinalizing(ctx context.Context, snap *crdv1beta1.QuestDBSnapshot) (ctrl.Result, error) {

	// Create the post-snapshot job
	job, err := r.buildPostSnapshotJob(ctx, snap)
	if err != nil {
		return ctrl.Result{}, err
	}
	if err = r.Create(ctx, &job); err != nil {
		if err == nil {
			r.Recorder.Eventf(snap, v1.EventTypeNormal, "SnapshotFinalizing", "Running 'SNAPSHOT COMPLETE;' for snapshot %s", snap.Name)
		}
		if apierrors.IsAlreadyExists(err) {
			err = r.Get(ctx, client.ObjectKey{Name: job.Name, Namespace: job.Namespace}, &job)
		}
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	// Check if the post-snapshot job is complete
	if job.Status.Succeeded == 1 {
		// Set the phase to succeeded
		snap.Status.Phase = crdv1beta1.SnapshotSucceeded
		if err = r.Status().Update(ctx, snap); err != nil {
			return ctrl.Result{}, err
		}
		r.Recorder.Eventf(snap, v1.EventTypeNormal, "SnapshotSucceeded", "Snapshot %s succeeded", snap.Name)
		return ctrl.Result{}, nil
	}

	// If we've reached the maximum number of attempts, fail the snapshot
	// todo: this is serious, since we are unable to take another snapshot without manual database action
	if job.Status.Failed >= snap.Spec.JobBackoffLimit {
		snap.Status.Phase = crdv1beta1.SnapshotFailed
		err = r.Status().Update(ctx, snap)
		if err != nil {
			return ctrl.Result{}, err
		}
		r.Recorder.Eventf(snap, v1.EventTypeWarning, "SnapshotFailed", "Error running 'SNAPSHOT COMPLETE' in job %s", job.Name)
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, err
}

func (r *QuestDBSnapshotReconciler) handlePhaseFailed(ctx context.Context, snap *crdv1beta1.QuestDBSnapshot) (ctrl.Result, error) {
	var err error
	// Remove the SnapshotCompleteFinalizer
	if controllerutil.ContainsFinalizer(snap, crdv1beta1.SnapshotCompleteFinalizer) {
		controllerutil.RemoveFinalizer(snap, crdv1beta1.SnapshotCompleteFinalizer)
		err = r.Update(ctx, snap)
	}
	return ctrl.Result{}, err
}

func (r *QuestDBSnapshotReconciler) handlePhaseSucceeded(ctx context.Context, snap *crdv1beta1.QuestDBSnapshot) (ctrl.Result, error) {
	var err error
	// Remove the SnapshotCompleteFinalizer
	if controllerutil.ContainsFinalizer(snap, crdv1beta1.SnapshotCompleteFinalizer) {
		controllerutil.RemoveFinalizer(snap, crdv1beta1.SnapshotCompleteFinalizer)
		err = r.Update(ctx, snap)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	// Delete the pre and post snapshot jobs
	job := &batchv1.Job{}
	for _, jobName := range []string{fmt.Sprintf("%s-pre-snapshot", snap.Name), fmt.Sprintf("%s-post-snapshot", snap.Name)} {
		err := r.Get(ctx, client.ObjectKey{Name: jobName, Namespace: snap.Namespace}, job)
		if apierrors.IsNotFound(err) {
			continue
		}
		if err != nil {
			return ctrl.Result{}, err
		}
		var propagationPolicy = metav1.DeletePropagationBackground
		if err = r.Delete(ctx, job, &client.DeleteOptions{
			PropagationPolicy: &propagationPolicy,
		}); err != nil {
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}
	}

	return ctrl.Result{}, nil
}
