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
	"time"

	appsv1 "k8s.io/api/apps/v1"
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
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	crdv1beta2 "github.com/questdb/questdb-operator/api/v1beta2"
)

const fieldOwner = client.FieldOwner("questdb-operator")

const (
	conditionReady          = "Ready"
	conditionCredentials    = "CredentialsReady"
	credentialsInvalidEvent = "CredentialsInvalid"
)

// QuestDBReconciler reconciles a QuestDB object.
type QuestDBReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=crd.questdb.io,resources=questdbs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=crd.questdb.io,resources=questdbs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=crd.questdb.io,resources=questdbs/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services;configmaps;persistentvolumeclaims;secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// Reconcile drives a QuestDB towards its desired state: credentials, then the data PVC, config,
// Service, and StatefulSet, then status.
func (r *QuestDBReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	qdb := &crdv1beta2.QuestDB{}
	if err := r.Get(ctx, req.NamespacedName, qdb); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Credentials: generate or resolve the referenced Secret.
	secret, err := r.reconcileCredentials(ctx, qdb)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return r.failCredentials(ctx, qdb, "credentials secret "+credentialsSecretName(qdb)+" not found")
		}
		return ctrl.Result{}, err
	}
	if err := validateCredentialsSecret(secret); err != nil {
		return r.failCredentials(ctx, qdb, err.Error())
	}
	credHash := hashCredentialsSecret(secret)

	// Data PVC (optionally restored from a VolumeSnapshot).
	pvc, err := r.desiredPVC(qdb)
	if err != nil {
		return ctrl.Result{}, err
	}
	if err := r.apply(ctx, pvc); err != nil {
		return ctrl.Result{}, err
	}

	// Config (ConfigMap is optional).
	cm, err := r.desiredConfigMap(qdb)
	if err != nil {
		return ctrl.Result{}, err
	}
	if cm != nil {
		if err := r.apply(ctx, cm); err != nil {
			return ctrl.Result{}, err
		}
	} else {
		// Config was removed: delete the operator's ConfigMap if it exists — but only if we own it,
		// so a pre-existing unrelated ConfigMap that happens to share the QuestDB's name is left alone.
		stale := &corev1.ConfigMap{}
		switch err := r.Get(ctx, types.NamespacedName{Name: qdb.Name, Namespace: qdb.Namespace}, stale); {
		case apierrors.IsNotFound(err):
			// nothing to delete
		case err != nil:
			return ctrl.Result{}, err
		case metav1.IsControlledBy(stale, qdb):
			if err := r.Delete(ctx, stale); err != nil && !apierrors.IsNotFound(err) {
				return ctrl.Result{}, err
			}
		}
	}

	// Service.
	svc, err := r.desiredService(qdb)
	if err != nil {
		return ctrl.Result{}, err
	}
	if err := r.apply(ctx, svc); err != nil {
		return ctrl.Result{}, err
	}

	// StatefulSet.
	sts, err := r.desiredStatefulSet(qdb, cm, secret.Name, credHash)
	if err != nil {
		return ctrl.Result{}, err
	}
	if err := r.apply(ctx, sts); err != nil {
		return ctrl.Result{}, err
	}

	log.V(1).Info("reconciled QuestDB resources", "credentialsSecret", secret.Name)
	return r.reconcileStatus(ctx, qdb, secret.Name)
}

// apply performs a server-side apply of obj owned by the operator.
func (r *QuestDBReconciler) apply(ctx context.Context, obj client.Object) error {
	gvk, err := apiutil.GVKForObject(obj, r.Scheme)
	if err != nil {
		return err
	}
	obj.GetObjectKind().SetGroupVersionKind(gvk)
	// Typed server-side apply via the patch API; the newer Client.Apply needs apply configurations.
	return r.Patch(ctx, obj, client.Apply, fieldOwner, client.ForceOwnership) //nolint:staticcheck
}

// reconcileStatus refreshes QuestDB status from the StatefulSet and updates conditions only when changed.
func (r *QuestDBReconciler) reconcileStatus(ctx context.Context, qdb *crdv1beta2.QuestDB, credSecretName string) (ctrl.Result, error) {
	sts := &appsv1.StatefulSet{}
	var ready int32
	if err := r.Get(ctx, types.NamespacedName{Name: qdb.Name, Namespace: qdb.Namespace}, sts); err == nil {
		ready = sts.Status.ReadyReplicas
	} else if !apierrors.IsNotFound(err) {
		return ctrl.Result{}, err
	}

	before := qdb.Status.DeepCopy()
	qdb.Status.ReadyReplicas = ready
	qdb.Status.CredentialsSecretName = credSecretName

	apimeta.SetStatusCondition(&qdb.Status.Conditions, metav1.Condition{
		Type: conditionCredentials, Status: metav1.ConditionTrue,
		Reason: "CredentialsResolved", Message: "pg-wire credentials are available",
		ObservedGeneration: qdb.Generation,
	})
	readyCond := metav1.Condition{Type: conditionReady, ObservedGeneration: qdb.Generation}
	if ready >= 1 {
		readyCond.Status = metav1.ConditionTrue
		readyCond.Reason = "StatefulSetReady"
		readyCond.Message = "QuestDB is ready"
	} else {
		readyCond.Status = metav1.ConditionFalse
		readyCond.Reason = "StatefulSetNotReady"
		readyCond.Message = "waiting for the QuestDB pod to become ready"
	}
	apimeta.SetStatusCondition(&qdb.Status.Conditions, readyCond)

	if !apiequality.Semantic.DeepEqual(before, &qdb.Status) {
		if err := r.Status().Update(ctx, qdb); err != nil {
			return ctrl.Result{}, err
		}
	}
	if ready < 1 {
		return ctrl.Result{RequeueAfter: 15 * time.Second}, nil
	}
	return ctrl.Result{}, nil
}

// failCredentials records a CredentialsReady=False condition and requeues; credential problems are a
// controller concern, not an admission rejection.
func (r *QuestDBReconciler) failCredentials(ctx context.Context, qdb *crdv1beta2.QuestDB, msg string) (ctrl.Result, error) {
	before := qdb.Status.DeepCopy()
	qdb.Status.CredentialsSecretName = credentialsSecretName(qdb)
	apimeta.SetStatusCondition(&qdb.Status.Conditions, metav1.Condition{
		Type: conditionCredentials, Status: metav1.ConditionFalse,
		Reason: "CredentialsInvalid", Message: msg, ObservedGeneration: qdb.Generation,
	})
	if !apiequality.Semantic.DeepEqual(before, &qdb.Status) {
		if err := r.Status().Update(ctx, qdb); err != nil {
			return ctrl.Result{}, err
		}
		r.Recorder.Event(qdb, corev1.EventTypeWarning, credentialsInvalidEvent, msg)
	}
	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

// questdbsForSecret maps a Secret to the QuestDBs that reference it (cached, indexed lookup).
func (r *QuestDBReconciler) questdbsForSecret(ctx context.Context, obj client.Object) []reconcile.Request {
	var list crdv1beta2.QuestDBList
	if err := r.List(ctx, &list,
		client.InNamespace(obj.GetNamespace()),
		client.MatchingFields{credentialsSecretField: obj.GetName()},
	); err != nil {
		return nil
	}
	reqs := make([]reconcile.Request, 0, len(list.Items))
	for i := range list.Items {
		reqs = append(reqs, reconcile.Request{NamespacedName: types.NamespacedName{
			Name: list.Items[i].Name, Namespace: list.Items[i].Namespace,
		}})
	}
	return reqs
}

// SetupWithManager sets up the controller with the Manager.
func (r *QuestDBReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &crdv1beta2.QuestDB{}, credentialsSecretField,
		func(obj client.Object) []string {
			qdb := obj.(*crdv1beta2.QuestDB)
			if qdb.Spec.Auth.Psql != nil && qdb.Spec.Auth.Psql.SecretName != "" {
				return []string{qdb.Spec.Auth.Psql.SecretName}
			}
			return nil
		}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&crdv1beta2.QuestDB{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.PersistentVolumeClaim{}).
		Owns(&corev1.Secret{}).
		Watches(&corev1.Secret{}, handler.EnqueueRequestsFromMapFunc(r.questdbsForSecret)).
		Named("questdb").
		Complete(r)
}
