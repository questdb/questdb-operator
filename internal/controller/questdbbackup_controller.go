/*
Copyright 2026.

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
	"time"

	volumesnapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v8/apis/volumesnapshot/v1"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	crdv1beta2 "github.com/questdb/questdb-operator/api/v1beta2"
)

const (
	// checkpointHoldDeadline bounds how long a checkpoint may stay open waiting for the volume
	// snapshot to become ready. Past this the backup fails (and releases the checkpoint), so a
	// stuck CSI snapshot can never pin the database in checkpoint mode indefinitely.
	checkpointHoldDeadline = 15 * time.Minute
	// snapshotPollInterval is how often a not-yet-ready VolumeSnapshot is re-checked.
	snapshotPollInterval = 10 * time.Second
	// checkpointReleaseBackoff is the slower retry cadence for CHECKPOINT RELEASE once the hold
	// deadline has passed, so a long database outage retries the (idempotent) release without
	// busy-looping. The release obligation is never abandoned: a Succeeded backup always means the
	// checkpoint was released.
	checkpointReleaseBackoff = time.Minute
	// dbOpTimeout bounds a single CHECKPOINT operation against QuestDB.
	dbOpTimeout = 30 * time.Second
	// deleteReleaseDeadline bounds how long deletion waits to release a checkpoint before the
	// finalizer is force-removed (e.g. when the database is permanently unreachable).
	deleteReleaseDeadline = 5 * time.Minute

	conditionBackupReady = "Ready"
)

// QuestDBBackupReconciler reconciles a QuestDBBackup object.
type QuestDBBackupReconciler struct {
	client.Client
	Scheme       *runtime.Scheme
	Recorder     record.EventRecorder
	Checkpointer Checkpointer
}

// +kubebuilder:rbac:groups=crd.questdb.io,resources=questdbbackups,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=crd.questdb.io,resources=questdbbackups/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=crd.questdb.io,resources=questdbbackups/finalizers,verbs=update
// +kubebuilder:rbac:groups=crd.questdb.io,resources=questdbs,verbs=get;list;watch
// +kubebuilder:rbac:groups=snapshot.storage.k8s.io,resources=volumesnapshots,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// Reconcile drives a QuestDBBackup through: checkpoint create -> volume snapshot -> checkpoint
// release -> succeeded, guaranteeing the checkpoint is released on every exit path.
func (r *QuestDBBackupReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	backup := &crdv1beta2.QuestDBBackup{}
	if err := r.Get(ctx, req.NamespacedName, backup); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !backup.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, backup)
	}

	// Add the finalizer up front so a delete at any point still runs checkpoint cleanup.
	if controllerutil.AddFinalizer(backup, crdv1beta2.BackupFinalizer) {
		if err := r.Update(ctx, backup); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	if backup.IsComplete() {
		return ctrl.Result{}, nil
	}

	// A QuestDB can hold only one checkpoint, so only the oldest in-flight backup for a given
	// QuestDB proceeds; the rest wait their turn.
	active, err := r.isActiveBackup(ctx, backup)
	if err != nil {
		return ctrl.Result{}, err
	}
	if !active {
		return ctrl.Result{RequeueAfter: snapshotPollInterval}, nil
	}

	qdb := &crdv1beta2.QuestDB{}
	if err := r.Get(ctx, types.NamespacedName{Name: backup.Spec.QuestDBName, Namespace: backup.Namespace}, qdb); err != nil {
		if apierrors.IsNotFound(err) {
			// If we hold a checkpoint, a NotFound may be a transient cache miss or a recreate; the
			// open checkpoint can only be released by reaching the database, so requeue and retry
			// until the hold deadline rather than failing terminally and stranding it.
			if backup.CheckpointOutstanding() && !r.holdDeadlineExceeded(backup) {
				r.Recorder.Event(backup, corev1.EventTypeWarning, "QuestDBNotFound",
					fmt.Sprintf("QuestDB %q not found while a checkpoint is outstanding; retrying", backup.Spec.QuestDBName))
				return ctrl.Result{RequeueAfter: snapshotPollInterval}, nil
			}
			return r.abort(ctx, backup, nil, "QuestDBNotFound", fmt.Sprintf("QuestDB %q not found", backup.Spec.QuestDBName))
		}
		return ctrl.Result{}, err
	}

	switch backup.Status.Phase {
	case "", crdv1beta2.BackupPhasePending:
		return r.reconcilePending(ctx, backup, qdb)
	case crdv1beta2.BackupPhaseCheckpointCreated:
		return r.reconcileCheckpointCreated(ctx, backup, qdb)
	case crdv1beta2.BackupPhaseSnapshotCreated:
		return r.reconcileSnapshotCreated(ctx, backup, qdb)
	default:
		log.Info("unknown backup phase, treating as failed", "phase", backup.Status.Phase)
		return r.abort(ctx, backup, qdb, "UnknownPhase", fmt.Sprintf("unknown phase %q", backup.Status.Phase))
	}
}

// reconcilePending issues CHECKPOINT CREATE and advances to CheckpointCreated.
func (r *QuestDBBackupReconciler) reconcilePending(ctx context.Context, backup *crdv1beta2.QuestDBBackup, qdb *crdv1beta2.QuestDB) (ctrl.Result, error) {
	// Fail fast if the data PVC the snapshot will source from is absent: opening a checkpoint first
	// would hold the database in checkpoint mode for the whole hold deadline waiting for a snapshot
	// that can never succeed.
	pvc := &corev1.PersistentVolumeClaim{}
	if err := r.Get(ctx, types.NamespacedName{Name: dataPVCName(backup.Spec.QuestDBName), Namespace: backup.Namespace}, pvc); err != nil {
		if apierrors.IsNotFound(err) {
			r.Recorder.Event(backup, corev1.EventTypeWarning, "WaitingForPVC",
				fmt.Sprintf("data PVC %q not found yet", dataPVCName(backup.Spec.QuestDBName)))
			return r.requeueOrAbort(ctx, backup, qdb, "PVCNotFound", "data PVC did not appear before the deadline")
		}
		return ctrl.Result{}, err
	}

	target, err := r.resolveTarget(ctx, qdb)
	if err != nil {
		r.Recorder.Event(backup, corev1.EventTypeWarning, "WaitingForQuestDB", err.Error())
		return r.requeueOrAbort(ctx, backup, qdb, "WaitingForQuestDB", err.Error())
	}

	// Record the release obligation BEFORE issuing CHECKPOINT CREATE. If the process crashes or the
	// status write fails after the database call, the persisted CheckpointCreatedAt still drives a
	// release on delete/abort (CHECKPOINT RELEASE is idempotent on QuestDB, so recording it slightly
	// ahead of the actual checkpoint is safe), and it anchors the hold deadline.
	if backup.Status.CheckpointCreatedAt == nil {
		now := metav1.Now()
		backup.Status.CheckpointCreatedAt = &now
		setBackupCondition(backup, metav1.ConditionFalse, "CheckpointCreating", "opening QuestDB checkpoint")
		if err := r.Status().Update(ctx, backup); err != nil {
			return ctrl.Result{}, err
		}
		// Fall through and issue CHECKPOINT CREATE in this same reconcile; the obligation is already
		// persisted, so a crash before the phase advances still leaves a release record.
	}

	opCtx, cancel := context.WithTimeout(ctx, dbOpTimeout)
	defer cancel()
	if err := r.Checkpointer.Create(opCtx, target); err != nil {
		// The pod may not be reachable yet; retry, but bound it so a permanently-unreachable database
		// can't keep this backup the perpetual "active" one and block every sibling backup forever.
		r.Recorder.Event(backup, corev1.EventTypeWarning, "CheckpointCreateFailed", err.Error())
		return r.requeueOrAbort(ctx, backup, qdb, "CheckpointCreateFailed", err.Error())
	}

	backup.Status.Phase = crdv1beta2.BackupPhaseCheckpointCreated
	setBackupCondition(backup, metav1.ConditionFalse, "CheckpointCreated", "checkpoint created; snapshotting volume")
	if err := r.Status().Update(ctx, backup); err != nil {
		return ctrl.Result{}, err
	}
	r.Recorder.Event(backup, corev1.EventTypeNormal, "CheckpointCreated", "QuestDB checkpoint created")
	return ctrl.Result{Requeue: true}, nil
}

// holdDeadlineExceeded reports whether the checkpoint hold deadline has passed, anchored on when the
// checkpoint was opened (or, before that, when the backup was created).
func (r *QuestDBBackupReconciler) holdDeadlineExceeded(backup *crdv1beta2.QuestDBBackup) bool {
	anchor := backup.CreationTimestamp.Time
	if backup.Status.CheckpointCreatedAt != nil {
		anchor = backup.Status.CheckpointCreatedAt.Time
	}
	return !anchor.IsZero() && time.Since(anchor) > checkpointHoldDeadline
}

// requeueOrAbort requeues a still-progressing backup, or aborts it once the hold deadline has passed
// so a stuck backup stops blocking siblings and any outstanding checkpoint is released.
func (r *QuestDBBackupReconciler) requeueOrAbort(ctx context.Context, backup *crdv1beta2.QuestDBBackup, qdb *crdv1beta2.QuestDB, reason, msg string) (ctrl.Result, error) {
	if r.holdDeadlineExceeded(backup) {
		return r.abort(ctx, backup, qdb, reason, "deadline exceeded: "+msg)
	}
	return ctrl.Result{RequeueAfter: snapshotPollInterval}, nil
}

// reconcileCheckpointCreated creates the VolumeSnapshot and waits for it to become ready.
func (r *QuestDBBackupReconciler) reconcileCheckpointCreated(ctx context.Context, backup *crdv1beta2.QuestDBBackup, qdb *crdv1beta2.QuestDB) (ctrl.Result, error) {
	vs, err := r.ensureVolumeSnapshot(ctx, backup)
	if err != nil {
		// A persistent snapshot-API error (e.g. the VolumeSnapshot CRD/RBAC is missing) must still be
		// bounded by the hold deadline, otherwise the checkpoint stays open indefinitely.
		if r.holdDeadlineExceeded(backup) {
			return r.abort(ctx, backup, qdb, "SnapshotError", "volume snapshot could not be created before the checkpoint hold deadline: "+err.Error())
		}
		return ctrl.Result{}, err
	}

	before := backup.Status.DeepCopy()
	backup.Status.VolumeSnapshotName = vs.Name

	if vs.Status != nil && vs.Status.Error != nil && vs.Status.Error.Message != nil {
		return r.abort(ctx, backup, qdb, "SnapshotFailed", *vs.Status.Error.Message)
	}

	if vs.Status != nil && vs.Status.ReadyToUse != nil && *vs.Status.ReadyToUse {
		backup.Status.Phase = crdv1beta2.BackupPhaseSnapshotCreated
		if err := r.Status().Update(ctx, backup); err != nil {
			return ctrl.Result{}, err
		}
		r.Recorder.Event(backup, corev1.EventTypeNormal, "SnapshotReady", "volume snapshot is ready")
		return ctrl.Result{Requeue: true}, nil
	}

	// Bound how long the checkpoint stays open while waiting for the snapshot.
	if r.holdDeadlineExceeded(backup) {
		return r.abort(ctx, backup, qdb, "SnapshotTimeout", "volume snapshot did not become ready before the checkpoint hold deadline")
	}

	if !apiequality.Semantic.DeepEqual(before, &backup.Status) {
		if err := r.Status().Update(ctx, backup); err != nil {
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{RequeueAfter: snapshotPollInterval}, nil
}

// reconcileSnapshotCreated issues CHECKPOINT RELEASE and marks the backup succeeded.
func (r *QuestDBBackupReconciler) reconcileSnapshotCreated(ctx context.Context, backup *crdv1beta2.QuestDBBackup, qdb *crdv1beta2.QuestDB) (ctrl.Result, error) {
	if backup.Status.CheckpointReleasedAt == nil {
		if err := r.releaseCheckpoint(ctx, qdb); err != nil {
			// The snapshot is captured, but the backup must not reach a terminal Succeeded state with
			// the checkpoint still open. Keep retrying the (idempotent) release until the database is
			// reachable; back off past the hold deadline so a long outage doesn't busy-loop. The
			// obligation is only otherwise cleared on delete (reconcileDelete).
			r.Recorder.Event(backup, corev1.EventTypeWarning, "CheckpointReleaseFailed", err.Error())
			requeue := snapshotPollInterval
			if r.holdDeadlineExceeded(backup) {
				requeue = checkpointReleaseBackoff
			}
			return ctrl.Result{RequeueAfter: requeue}, nil
		}
		now := metav1.Now()
		backup.Status.CheckpointReleasedAt = &now
	}
	backup.Status.Phase = crdv1beta2.BackupPhaseSucceeded
	setBackupCondition(backup, metav1.ConditionTrue, "BackupComplete", "backup complete")
	if err := r.Status().Update(ctx, backup); err != nil {
		return ctrl.Result{}, err
	}
	r.Recorder.Event(backup, corev1.EventTypeNormal, "BackupSucceeded", "backup complete: "+backup.Status.VolumeSnapshotName)
	return ctrl.Result{}, nil
}

// reconcileDelete releases an outstanding checkpoint (best effort, bounded) before removing the finalizer.
func (r *QuestDBBackupReconciler) reconcileDelete(ctx context.Context, backup *crdv1beta2.QuestDBBackup) (ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(backup, crdv1beta2.BackupFinalizer) {
		return ctrl.Result{}, nil
	}

	if backup.CheckpointOutstanding() {
		qdb := &crdv1beta2.QuestDB{}
		err := r.Get(ctx, types.NamespacedName{Name: backup.Spec.QuestDBName, Namespace: backup.Namespace}, qdb)
		switch {
		case apierrors.IsNotFound(err):
			// The QuestDB (and therefore its checkpoint) is gone; nothing to release.
		case err != nil:
			return ctrl.Result{}, err
		default:
			if rerr := r.releaseCheckpoint(ctx, qdb); rerr != nil {
				// Don't block deletion forever if the database is unreachable.
				if !backup.DeletionTimestamp.IsZero() && time.Since(backup.DeletionTimestamp.Time) > deleteReleaseDeadline {
					r.Recorder.Event(backup, corev1.EventTypeWarning, "CheckpointReleaseAbandoned",
						"giving up releasing checkpoint after deadline: "+rerr.Error())
				} else {
					r.Recorder.Event(backup, corev1.EventTypeWarning, "CheckpointReleaseFailed", rerr.Error())
					return ctrl.Result{RequeueAfter: snapshotPollInterval}, nil
				}
			}
		}
	}

	controllerutil.RemoveFinalizer(backup, crdv1beta2.BackupFinalizer)
	return ctrl.Result{}, r.Update(ctx, backup)
}

// abort releases an outstanding checkpoint (retrying until it succeeds) and then marks the backup failed.
func (r *QuestDBBackupReconciler) abort(ctx context.Context, backup *crdv1beta2.QuestDBBackup, qdb *crdv1beta2.QuestDB, reason, msg string) (ctrl.Result, error) {
	if backup.CheckpointOutstanding() && qdb != nil {
		if err := r.releaseCheckpoint(ctx, qdb); err != nil {
			r.Recorder.Event(backup, corev1.EventTypeWarning, "CheckpointReleaseFailed", err.Error())
			return ctrl.Result{RequeueAfter: snapshotPollInterval}, nil
		}
		now := metav1.Now()
		backup.Status.CheckpointReleasedAt = &now
	}
	backup.Status.Phase = crdv1beta2.BackupPhaseFailed
	setBackupCondition(backup, metav1.ConditionFalse, reason, msg)
	if err := r.Status().Update(ctx, backup); err != nil {
		return ctrl.Result{}, err
	}
	r.Recorder.Event(backup, corev1.EventTypeWarning, reason, msg)
	return ctrl.Result{}, nil
}

// releaseCheckpoint runs CHECKPOINT RELEASE against the QuestDB (DB-only; the caller records status).
func (r *QuestDBBackupReconciler) releaseCheckpoint(ctx context.Context, qdb *crdv1beta2.QuestDB) error {
	target, err := r.resolveTarget(ctx, qdb)
	if err != nil {
		return err
	}
	opCtx, cancel := context.WithTimeout(ctx, dbOpTimeout)
	defer cancel()
	return r.Checkpointer.Release(opCtx, target)
}

// resolveTarget builds the pg-wire connection details for a QuestDB from its credentials Secret.
func (r *QuestDBBackupReconciler) resolveTarget(ctx context.Context, qdb *crdv1beta2.QuestDB) (QuestDBConn, error) {
	secret := &corev1.Secret{}
	if err := r.Get(ctx, types.NamespacedName{Name: credentialsSecretName(qdb), Namespace: qdb.Namespace}, secret); err != nil {
		return QuestDBConn{}, err
	}
	if err := validateCredentialsSecret(secret); err != nil {
		return QuestDBConn{}, err
	}
	return QuestDBConn{
		Host:     fmt.Sprintf("%s.%s.svc", qdb.Name, qdb.Namespace),
		Port:     portPgWire,
		User:     string(secret.Data[envPgUser]),
		Password: string(secret.Data[envPgPassword]),
	}, nil
}

// ensureVolumeSnapshot creates the VolumeSnapshot of the QuestDB data PVC if absent, and returns it.
func (r *QuestDBBackupReconciler) ensureVolumeSnapshot(ctx context.Context, backup *crdv1beta2.QuestDBBackup) (*volumesnapshotv1.VolumeSnapshot, error) {
	key := types.NamespacedName{Name: backup.Name, Namespace: backup.Namespace}
	vs := &volumesnapshotv1.VolumeSnapshot{}
	err := r.Get(ctx, key, vs)
	if err == nil {
		return vs, nil
	}
	if !apierrors.IsNotFound(err) {
		return nil, err
	}

	pvcName := dataPVCName(backup.Spec.QuestDBName)
	vs = &volumesnapshotv1.VolumeSnapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name:      backup.Name,
			Namespace: backup.Namespace,
			Labels:    labelsForQuestDB(backup.Spec.QuestDBName),
		},
		Spec: volumesnapshotv1.VolumeSnapshotSpec{
			Source: volumesnapshotv1.VolumeSnapshotSource{
				PersistentVolumeClaimName: &pvcName,
			},
			VolumeSnapshotClassName: backup.Spec.VolumeSnapshotClassName,
		},
	}
	if err := controllerutil.SetControllerReference(backup, vs, r.Scheme); err != nil {
		return nil, err
	}
	if err := r.Create(ctx, vs); err != nil {
		if apierrors.IsAlreadyExists(err) {
			return vs, r.Get(ctx, key, vs)
		}
		return nil, err
	}
	r.Recorder.Event(backup, corev1.EventTypeNormal, "SnapshotCreating", "creating volume snapshot "+vs.Name)
	return vs, nil
}

// isActiveBackup reports whether this backup is the oldest in-flight backup for its QuestDB.
func (r *QuestDBBackupReconciler) isActiveBackup(ctx context.Context, backup *crdv1beta2.QuestDBBackup) (bool, error) {
	var list crdv1beta2.QuestDBBackupList
	if err := r.List(ctx, &list, client.InNamespace(backup.Namespace)); err != nil {
		return false, err
	}
	var oldest *crdv1beta2.QuestDBBackup
	for i := range list.Items {
		b := &list.Items[i]
		if b.Spec.QuestDBName != backup.Spec.QuestDBName || b.IsComplete() || !b.DeletionTimestamp.IsZero() {
			continue
		}
		if oldest == nil || isOlder(b, oldest) {
			oldest = b
		}
	}
	return oldest != nil && oldest.Name == backup.Name, nil
}

func isOlder(a, b *crdv1beta2.QuestDBBackup) bool {
	if a.CreationTimestamp.Equal(&b.CreationTimestamp) {
		return a.Name < b.Name
	}
	return a.CreationTimestamp.Before(&b.CreationTimestamp)
}

func setBackupCondition(backup *crdv1beta2.QuestDBBackup, status metav1.ConditionStatus, reason, msg string) {
	apimeta.SetStatusCondition(&backup.Status.Conditions, metav1.Condition{
		Type:               conditionBackupReady,
		Status:             status,
		Reason:             reason,
		Message:            msg,
		ObservedGeneration: backup.Generation,
	})
}

// SetupWithManager sets up the controller with the Manager.
func (r *QuestDBBackupReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.Checkpointer == nil {
		r.Checkpointer = NewCheckpointer()
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&crdv1beta2.QuestDBBackup{}).
		Owns(&volumesnapshotv1.VolumeSnapshot{}).
		Named("questdbbackup").
		Complete(r)
}
