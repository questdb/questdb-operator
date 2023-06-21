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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
		err error

		sched      = &crdv1beta1.QuestDBSnapshotSchedule{}
		latestSnap = &crdv1beta1.QuestDBSnapshot{}
		_          = log.FromContext(ctx)
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

	// Get the latest snapshot, if it exists
	if latestSnap, err = r.getLatest(ctx, sched); err != nil {
		return ctrl.Result{}, err
	}

	// Update the snapshot phase status based on the latest snapshot
	if latestSnap != nil {
		if latestSnap.Status.Phase != sched.Status.SnapshotPhase {
			sched.Status.SnapshotPhase = latestSnap.Status.Phase
			if err = r.Status().Update(ctx, sched); err != nil {
				return ctrl.Result{}, err
			}
		}
	}

	// Check if we are due for a snapshot
	nextSnapshotTime, err := r.getNextSnapshotTime(sched)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Calculate the requeue time before any modifications
	// are made to the NextSnapshot time
	requeueTime := nextSnapshotTime.Sub(r.TimeSource.Now())
	if requeueTime < 0 {
		requeueTime = 0
	}

	if nextSnapshotTime.Before(r.TimeSource.Now()) {
		// Update the next snapshot time
		sched.Status.NextSnapshot = metav1.NewTime(nextSnapshotTime)
		if err = r.Status().Update(ctx, sched); err != nil {
			return ctrl.Result{}, err
		}

		// Skip taking a snapshot if the latest snapshot is not complete
		if latestSnap != nil && latestSnap.Status.Phase != crdv1beta1.SnapshotSucceeded {
			r.Recorder.Event(sched, "Warning", "SnapshotSkipped", fmt.Sprintf("Skipping snapshot because the latest snapshot is not complete: %s", latestSnap.Name))
			return ctrl.Result{}, nil
		}

		// Build the snapshot
		snap := r.buildSnapshot(sched)

		err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
			// Create the snapshot
			if err = r.Create(ctx, &snap); err != nil {
				if !apierrors.IsAlreadyExists(err) {
					r.Recorder.Event(sched, "Warning", "SnapshotFailed", fmt.Sprintf("Failed to create snapshot: %s", err))
				}
			}

			return err
		})

		if err == nil {
			r.Recorder.Event(sched, "Normal", "SnapshotCreated", fmt.Sprintf("Created snapshot: %s", snap.Name))
		}

	}

	return ctrl.Result{RequeueAfter: requeueTime}, client.IgnoreAlreadyExists(err)
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

func (r *QuestDBSnapshotScheduleReconciler) getLatest(ctx context.Context, sched *crdv1beta1.QuestDBSnapshotSchedule) (*crdv1beta1.QuestDBSnapshot, error) {
	var (
		err error

		snapList = &crdv1beta1.QuestDBSnapshotList{}
	)

	if err = r.List(ctx, snapList, client.InNamespace(sched.Namespace)); err != nil {
		return nil, err
	}

	// Sort in descending order so we can garbage collect
	sort.Slice(snapList.Items, func(i, j int) bool {
		return !snapList.Items[i].CreationTimestamp.Before(&snapList.Items[j].CreationTimestamp)
	})

	if len(snapList.Items) == 0 {
		return nil, nil
	}

	for idx, s := range snapList.Items {
		if idx >= int(sched.Spec.Retention) {
			if err = r.Delete(ctx, &s); err != nil {
				return nil, err
			}
			r.Recorder.Event(sched, "Normal", "SnapshotDeleted", fmt.Sprintf("Deleted snapshot: %s", s.Name))
		}
	}

	return &snapList.Items[0], nil

}

func (r *QuestDBSnapshotScheduleReconciler) getNextSnapshotTime(sched *crdv1beta1.QuestDBSnapshotSchedule) (time.Time, error) {
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	crontab, err := parser.Parse(sched.Spec.Schedule)
	if err != nil {
		return time.Time{}, err
	}

	lastTime := sched.Status.NextSnapshot.Time
	if lastTime.IsZero() {
		lastTime = sched.CreationTimestamp.Time
	}
	return crontab.Next(lastTime), nil
}
