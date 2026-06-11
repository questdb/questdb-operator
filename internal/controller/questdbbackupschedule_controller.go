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
	"sort"
	"time"

	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	crdv1beta2 "github.com/questdb/questdb-operator/api/v1beta2"
)

// QuestDBBackupScheduleReconciler reconciles a QuestDBBackupSchedule object.
type QuestDBBackupScheduleReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
	// Now returns the current time; overridable in tests. Defaults to time.Now.
	Now func() time.Time
}

// +kubebuilder:rbac:groups=crd.questdb.io,resources=questdbbackupschedules,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=crd.questdb.io,resources=questdbbackupschedules/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=crd.questdb.io,resources=questdbbackupschedules/finalizers,verbs=update
// +kubebuilder:rbac:groups=crd.questdb.io,resources=questdbbackups,verbs=get;list;watch;create;delete
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// Reconcile creates backups on the cron schedule and prunes old successful backups.
func (r *QuestDBBackupScheduleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	schedule := &crdv1beta2.QuestDBBackupSchedule{}
	if err := r.Get(ctx, req.NamespacedName, schedule); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	cronSchedule, err := crdv1beta2.ScheduleCronParser.Parse(schedule.Spec.Schedule)
	if err != nil {
		r.Recorder.Event(schedule, corev1.EventTypeWarning, "InvalidSchedule", fmt.Sprintf("invalid cron schedule %q: %v", schedule.Spec.Schedule, err))
		return r.updateStatus(ctx, schedule, nil, metav1.Condition{
			Type: "Ready", Status: metav1.ConditionFalse, Reason: "InvalidSchedule",
			Message: "cron schedule is invalid", ObservedGeneration: schedule.Generation,
		}, nil)
	}

	backups, err := r.orderedChildBackups(ctx, schedule)
	if err != nil {
		return ctrl.Result{}, err
	}
	var latest *crdv1beta2.QuestDBBackup
	if len(backups) > 0 {
		latest = backups[0]
	}

	now := r.now()
	last := schedule.CreationTimestamp.Time
	if schedule.Status.LastBackupTime != nil {
		last = schedule.Status.LastBackupTime.Time
	}

	created := false
	if !schedule.Spec.Suspend && !now.Before(cronSchedule.Next(last)) {
		switch {
		case latest != nil && !latest.IsComplete():
			r.Recorder.Event(schedule, corev1.EventTypeNormal, "BackupSkipped",
				fmt.Sprintf("skipping: previous backup %q is still in progress", latest.Name))
		default:
			backup, cerr := r.createBackup(ctx, schedule, now)
			if cerr != nil {
				r.Recorder.Event(schedule, corev1.EventTypeWarning, "BackupCreateFailed", cerr.Error())
				return ctrl.Result{}, cerr
			}
			created = true
			latest = backup
			backups = append([]*crdv1beta2.QuestDBBackup{backup}, backups...)
			r.Recorder.Event(schedule, corev1.EventTypeNormal, "BackupCreated", "created backup "+backup.Name)
		}
	}

	if err := r.pruneBackups(ctx, schedule, backups); err != nil {
		return ctrl.Result{}, err
	}

	ready := metav1.Condition{
		Type: "Ready", Status: metav1.ConditionTrue, Reason: "Scheduled",
		Message: "schedule is active", ObservedGeneration: schedule.Generation,
	}
	if schedule.Spec.Suspend {
		ready.Reason = "Suspended"
		ready.Message = "schedule is suspended"
	}
	var lastBackup *metav1.Time
	if created {
		t := metav1.NewTime(now)
		lastBackup = &t
	}
	if _, err := r.updateStatus(ctx, schedule, latest, ready, lastBackup); err != nil {
		return ctrl.Result{}, err
	}

	// A suspended schedule needs no cron-cadence requeue: the spec watch re-triggers when it is
	// unsuspended, and the child watch re-triggers on backup changes (for pruning). Requeuing on the
	// cron interval would just be periodic no-op reconciles.
	if schedule.Spec.Suspend {
		return ctrl.Result{}, nil
	}
	requeue := cronSchedule.Next(now).Sub(now)
	if requeue <= 0 {
		requeue = time.Second
	}
	return ctrl.Result{RequeueAfter: requeue}, nil
}

// createBackup instantiates a QuestDBBackup from the schedule's template.
func (r *QuestDBBackupScheduleReconciler) createBackup(ctx context.Context, schedule *crdv1beta2.QuestDBBackupSchedule, now time.Time) (*crdv1beta2.QuestDBBackup, error) {
	backup := &crdv1beta2.QuestDBBackup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", schedule.Name, now.UTC().Format("20060102150405")),
			Namespace: schedule.Namespace,
			Labels:    labelsForQuestDB(schedule.Spec.Backup.QuestDBName),
		},
		Spec: schedule.Spec.Backup,
	}
	if err := controllerutil.SetControllerReference(schedule, backup, r.Scheme); err != nil {
		return nil, err
	}
	if err := r.Create(ctx, backup); err != nil {
		if apierrors.IsAlreadyExists(err) {
			return backup, nil
		}
		return nil, err
	}
	return backup, nil
}

// pruneBackups deletes successful backups beyond the retention count (newest kept). Backups are
// owner-referenced, so deleting a backup garbage-collects its VolumeSnapshot.
func (r *QuestDBBackupScheduleReconciler) pruneBackups(ctx context.Context, schedule *crdv1beta2.QuestDBBackupSchedule, ordered []*crdv1beta2.QuestDBBackup) error {
	retention := schedule.Spec.Retention
	if retention < 0 {
		// A negative retention disables pruning entirely (keep all successful backups).
		return nil
	}
	if retention == 0 {
		retention = crdv1beta2.DefaultRetention
	}
	var kept int32
	for _, b := range ordered {
		if b.Status.Phase != crdv1beta2.BackupPhaseSucceeded || !b.DeletionTimestamp.IsZero() {
			continue
		}
		kept++
		if kept > retention {
			if err := r.Delete(ctx, b); err != nil && !apierrors.IsNotFound(err) {
				return err
			}
			r.Recorder.Event(schedule, corev1.EventTypeNormal, "BackupPruned", "deleted old backup "+b.Name)
		}
	}
	return nil
}

// orderedChildBackups returns the schedule's backups, newest first.
func (r *QuestDBBackupScheduleReconciler) orderedChildBackups(ctx context.Context, schedule *crdv1beta2.QuestDBBackupSchedule) ([]*crdv1beta2.QuestDBBackup, error) {
	var list crdv1beta2.QuestDBBackupList
	if err := r.List(ctx, &list, client.InNamespace(schedule.Namespace)); err != nil {
		return nil, err
	}
	owned := make([]*crdv1beta2.QuestDBBackup, 0, len(list.Items))
	for i := range list.Items {
		if metav1.IsControlledBy(&list.Items[i], schedule) {
			owned = append(owned, &list.Items[i])
		}
	}
	// Newest first, with a deterministic Name tiebreak for same-timestamp backups (schedule names are
	// second-granularity, so ties are common); this matches the backup controller's isOlder ordering
	// so prune and active-backup selection agree on which backup is newest/oldest.
	sort.SliceStable(owned, func(i, j int) bool {
		ti, tj := owned[i].CreationTimestamp, owned[j].CreationTimestamp
		if ti.Equal(&tj) {
			return owned[i].Name > owned[j].Name
		}
		return tj.Before(&ti)
	})
	return owned, nil
}

// updateStatus refreshes schedule status (mirroring the latest backup, optionally stamping
// LastBackupTime) and persists only on change.
func (r *QuestDBBackupScheduleReconciler) updateStatus(ctx context.Context, schedule *crdv1beta2.QuestDBBackupSchedule, latest *crdv1beta2.QuestDBBackup, ready metav1.Condition, lastBackupTime *metav1.Time) (ctrl.Result, error) {
	before := schedule.Status.DeepCopy()
	if latest != nil {
		schedule.Status.LastBackupName = latest.Name
		schedule.Status.LastBackupPhase = latest.Status.Phase
	}
	apimeta.SetStatusCondition(&schedule.Status.Conditions, ready)
	if lastBackupTime != nil {
		schedule.Status.LastBackupTime = lastBackupTime
	}
	if !apiequality.Semantic.DeepEqual(before, &schedule.Status) {
		if err := r.Status().Update(ctx, schedule); err != nil {
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}

func (r *QuestDBBackupScheduleReconciler) now() time.Time {
	if r.Now != nil {
		return r.Now()
	}
	return time.Now()
}

// SetupWithManager sets up the controller with the Manager.
func (r *QuestDBBackupScheduleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&crdv1beta2.QuestDBBackupSchedule{}).
		Owns(&crdv1beta2.QuestDBBackup{}).
		Named("questdbbackupschedule").
		Complete(r)
}
