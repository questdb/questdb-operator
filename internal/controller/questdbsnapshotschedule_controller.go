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
	"sort"
	"time"

	"github.com/thejerf/abtime"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	cron "github.com/robfig/cron/v3"

	crdv1beta1 "github.com/questdb/questdb-operator/api/v1beta1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// QuestDBSnapshotScheduleReconciler reconciles a QuestDBSnapshotSchedule object
type QuestDBSnapshotScheduleReconciler struct {
	client.Client
	Scheme     *runtime.Scheme
	Recorder   record.EventRecorder
	TimeSource abtime.AbstractTime
}

//+kubebuilder:rbac:groups=crd.questdb.io,resources=questdbsnapshotschedules,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=crd.questdb.io,resources=questdbsnapshotschedules/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=crd.questdb.io,resources=questdbsnapshotschedules/finalizers,verbs=update
//+kubebuilder:rbac:groups=questdbsnapshot.storage.k8s.io,resources=volumesnapshots,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *QuestDBSnapshotScheduleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var (
		err        error
		latestSnap = &crdv1beta1.QuestDBSnapshot{}

		sched          = &crdv1beta1.QuestDBSnapshotSchedule{}
		childSnapshots = &crdv1beta1.QuestDBSnapshotList{}
		_              = log.FromContext(ctx)
	)

	// Try to get the object we are reconciling.  Exit if it does not exist
	if err = r.Get(ctx, req.NamespacedName, sched); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Set retention if it is not set
	if sched.Spec.Retention == 0 {
		sched.Spec.Retention = crdv1beta1.ScheduleRetentionDefault
		if err = r.Update(ctx, sched); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Get child snapshots
	if childSnapshots, err = r.getOrderedChildSnapshots(ctx, sched); err != nil {
		return ctrl.Result{}, err
	}

	// Since snapshots are sorted in descending order, the latest snapshot is the first item
	if len(childSnapshots.Items) > 0 {
		latestSnap = &childSnapshots.Items[0]
	}

	// Update the snapshot phase status based on the latest snapshot
	if latestSnap.Status.Phase != sched.Status.SnapshotPhase {
		sched.Status.SnapshotPhase = latestSnap.Status.Phase
		if err = r.Status().Update(ctx, sched); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Check if we are due for a snapshot
	lastSnapshotTime, err := r.getLastSnapshotTime(sched)
	if err != nil {
		return ctrl.Result{}, err
	}

	if lastSnapshotTime.Compare(r.TimeSource.Now()) <= 0 {
		// Update the next snapshot time
		sched.Status.LastSnapshot = metav1.NewTime(lastSnapshotTime)
		if err = r.Status().Update(ctx, sched); err != nil {
			// If this update fails, re-reconcile immediately
			return ctrl.Result{}, err
		}

		// Skip taking a snapshot if the latest snapshot is not complete (succeeded or failed or empty phase)
		if sched.Status.SnapshotPhase != "" && !latestSnap.IsComplete() {
			r.Recorder.Event(sched, "Warning", "SnapshotSkipped", fmt.Sprintf("Skipping snapshot because the latest snapshot is not complete: %s", latestSnap.Name))
		} else {
			// Otherwise, build the snapshot
			snap := r.buildSnapshot(sched)

			// Create the snapshot, retrying on errors to ensure its creation
			err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
				return client.IgnoreAlreadyExists(r.Create(ctx, &snap))
			})

			if err == nil {
				r.Recorder.Event(sched, "Normal", "SnapshotCreated", fmt.Sprintf("Created snapshot: %s", snap.Name))

				// Add the new snapshot to the top of the child snapshot list
				childSnapshots.Items = append([]crdv1beta1.QuestDBSnapshot{snap}, childSnapshots.Items...)
			} else {
				r.Recorder.Event(sched, "Warning", "SnapshotFailed", fmt.Sprintf("Failed to create snapshot: %s", err))
			}
		}
	}

	// Garbage collect old successful snapshots
	err = r.garbageCollect(ctx, childSnapshots, sched.Spec.Retention)

	return ctrl.Result{RequeueAfter: lastSnapshotTime.Sub(r.TimeSource.Now())}, err
}

// SetupWithManager sets up the controller with the Manager.
func (r *QuestDBSnapshotScheduleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&crdv1beta1.QuestDBSnapshotSchedule{}).
		Owns(&crdv1beta1.QuestDBSnapshot{}).
		Complete(r)
}

func (r *QuestDBSnapshotScheduleReconciler) buildSnapshot(sched *crdv1beta1.QuestDBSnapshotSchedule) crdv1beta1.QuestDBSnapshot {
	var (
		err error
	)
	snap := crdv1beta1.QuestDBSnapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", sched.Name, r.TimeSource.Now().Format("20060102150405")),
			Namespace: sched.Namespace,
			Labels:    sched.Labels,
		},
		Spec: sched.Spec.Snapshot,
	}

	if err = ctrl.SetControllerReference(sched, &snap, r.Scheme); err != nil {
		panic(fmt.Sprintf("failed to set controller reference, even though we are building an object from scratch: %s", err.Error()))
	}

	return snap

}

func (r *QuestDBSnapshotScheduleReconciler) getOrderedChildSnapshots(ctx context.Context, sched *crdv1beta1.QuestDBSnapshotSchedule) (*crdv1beta1.QuestDBSnapshotList, error) {
	var (
		err error

		snapList  = &crdv1beta1.QuestDBSnapshotList{}
		ownedList = &crdv1beta1.QuestDBSnapshotList{}
	)

	if err = r.List(ctx, snapList, client.InNamespace(sched.Namespace)); err != nil {
		return nil, err
	}

	// Sort in descending order
	sort.Slice(snapList.Items, func(i, j int) bool {
		return !snapList.Items[i].CreationTimestamp.Before(&snapList.Items[j].CreationTimestamp)
	})

	for _, s := range snapList.Items {
		if metav1.IsControlledBy(&s, sched) {
			ownedList.Items = append(ownedList.Items, s)
		}
	}

	return ownedList, nil
}

// garbageCollect assumes that snapshots are ordered descending by creation timestamp
func (r *QuestDBSnapshotScheduleReconciler) garbageCollect(ctx context.Context, s *crdv1beta1.QuestDBSnapshotList, retention int32) error {
	var (
		err          error
		successCount int32
	)

	for _, snap := range s.Items {
		if snap.Status.Phase == crdv1beta1.SnapshotSucceeded {
			successCount++
		}

		// Delete snapshots that are older than the retention
		if successCount > retention && snap.Status.Phase == crdv1beta1.SnapshotSucceeded {
			if err = r.Delete(ctx, &snap); err != nil {
				return err
			}
			r.Recorder.Event(s, "Normal", "SnapshotDeleted", fmt.Sprintf("Deleted snapshot: %s", snap.Name))
		}
	}

	return nil

}

func (r *QuestDBSnapshotScheduleReconciler) getLastSnapshotTime(sched *crdv1beta1.QuestDBSnapshotSchedule) (time.Time, error) {
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	crontab, err := parser.Parse(sched.Spec.Schedule)
	if err != nil {
		return time.Time{}, err
	}

	lastTime := sched.Status.LastSnapshot.Time
	if lastTime.IsZero() {
		lastTime = sched.CreationTimestamp.Time
	}
	return crontab.Next(lastTime), nil
}
