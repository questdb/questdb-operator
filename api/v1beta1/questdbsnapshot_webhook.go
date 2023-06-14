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

package v1beta1

import (
	"errors"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

const (
	JobBackoffLimitDefault int32 = 5
)

// log is for logging in this package.
var questdbsnapshotlog = logf.Log.WithName("questdbsnapshot-resource")

func (r *QuestDBSnapshot) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//+kubebuilder:webhook:path=/mutate-crd-questdb-io-v1beta1-questdbsnapshot,mutating=true,failurePolicy=fail,sideEffects=None,groups=crd.questdb.io,resources=questdbsnapshots,verbs=create;update,versions=v1beta1,name=mquestdbsnapshot.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &QuestDBSnapshot{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *QuestDBSnapshot) Default() {
	questdbsnapshotlog.Info("default", "name", r.Name)

	if r.Spec.JobBackoffLimit == 0 {
		r.Spec.JobBackoffLimit = JobBackoffLimitDefault
	}
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-crd-questdb-io-v1beta1-questdbsnapshot,mutating=false,failurePolicy=fail,sideEffects=None,groups=crd.questdb.io,resources=questdbsnapshots,verbs=create;update,versions=v1beta1,name=vquestdbsnapshot.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &QuestDBSnapshot{}

func validateSnapshotCreate(spec QuestDBSnapshotSpec) error {
	if spec.QuestDBName == "" {
		return errors.New("QuestDBName is required")
	}

	if spec.VolumeSnapshotClassName == "" {
		return errors.New("VolumeSnapshotClassName is required")
	}

	if spec.JobBackoffLimit <= 0 {
		return errors.New("JobBackoffLimit must be greater than 0")
	}

	return nil
}

func validateSnapshotUpdate(oldSpec, newSpec QuestDBSnapshotSpec) error {
	if newSpec.QuestDBName != oldSpec.QuestDBName {
		return errors.New("QuestDBName is immutable")
	}

	if newSpec.VolumeSnapshotClassName != oldSpec.VolumeSnapshotClassName {
		return errors.New("VolumeSnapshotClassName is immutable")
	}

	if newSpec.JobBackoffLimit != oldSpec.JobBackoffLimit {
		return errors.New("JobBackoffLimit is immutable")
	}
	return nil
}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *QuestDBSnapshot) ValidateCreate() error {
	questdbsnapshotlog.Info("validate create", "name", r.Name)
	return validateSnapshotCreate(r.Spec)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *QuestDBSnapshot) ValidateUpdate(old runtime.Object) error {
	questdbsnapshotlog.Info("validate update", "name", r.Name)

	oldSnap, valid := old.(*QuestDBSnapshot)
	if !valid {
		return errors.New("old object is not a QuestDBSnapshot")
	}

	return validateSnapshotUpdate(oldSnap.Spec, r.Spec)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *QuestDBSnapshot) ValidateDelete() error {
	questdbsnapshotlog.Info("validate delete", "name", r.Name)

	// Nothing to do here. Finalizers handle the deletion process

	return nil
}
