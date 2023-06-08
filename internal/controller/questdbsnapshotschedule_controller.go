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
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	crdv1beta1 "github.com/questdb/questdb-operator/api/v1beta1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// QuestDBSnapshotScheduleReconciler reconciles a QuestDBSnapshotSchedule object
type QuestDBSnapshotScheduleReconciler struct {
	client.Client
	Scheme *runtime.Scheme
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

		sched = &crdv1beta1.QuestDBSnapshotSchedule{}
		_     = log.FromContext(ctx)
	)

	// Try to get the object we are reconciling.  Exit if it does not exist
	if err = r.Get(ctx, req.NamespacedName, sched); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *QuestDBSnapshotScheduleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&crdv1beta1.QuestDBSnapshotSchedule{}).
		Owns(&crdv1beta1.QuestDBSnapshot{}).
		Complete(r)
}

func (r *QuestDBSnapshotScheduleReconciler) buildSnapshot(sched *crdv1beta1.QuestDBSnapshotSchedule) (crdv1beta1.QuestDBSnapshot, error) {
	var (
		err error
	)
	snap := crdv1beta1.QuestDBSnapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", sched.Name, time.Now().Format("20060102150405")),
			Namespace: sched.Namespace,
			Labels:    sched.Labels,
		},
		Spec: sched.Spec.Snapshot,
	}

	err = ctrl.SetControllerReference(sched, &snap, r.Scheme)
	return snap, err

}