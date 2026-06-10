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

package v1beta1

import (
	"context"

	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	crdv1beta1 "github.com/questdb/questdb-operator/api/v1beta1"
)

// nolint:unused
// log is for logging in this package.
var questdbsnapshotschedulelog = logf.Log.WithName("questdbsnapshotschedule-resource")

// SetupQuestDBSnapshotScheduleWebhookWithManager registers the webhook for QuestDBSnapshotSchedule in the manager.
func SetupQuestDBSnapshotScheduleWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &crdv1beta1.QuestDBSnapshotSchedule{}).
		WithValidator(&QuestDBSnapshotScheduleCustomValidator{}).
		WithDefaulter(&QuestDBSnapshotScheduleCustomDefaulter{}).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

// +kubebuilder:webhook:path=/mutate-crd-questdb-io-v1beta1-questdbsnapshotschedule,mutating=true,failurePolicy=fail,sideEffects=None,groups=crd.questdb.io,resources=questdbsnapshotschedules,verbs=create;update,versions=v1beta1,name=mquestdbsnapshotschedule-v1beta1.kb.io,admissionReviewVersions=v1

// QuestDBSnapshotScheduleCustomDefaulter struct is responsible for setting default values on the custom resource of the
// Kind QuestDBSnapshotSchedule when those are created or updated.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as it is used only for temporary operations and does not need to be deeply copied.
type QuestDBSnapshotScheduleCustomDefaulter struct {
	// TODO(user): Add more fields as needed for defaulting
}

// Default implements webhook.CustomDefaulter so a webhook will be registered for the Kind QuestDBSnapshotSchedule.
func (d *QuestDBSnapshotScheduleCustomDefaulter) Default(_ context.Context, obj *crdv1beta1.QuestDBSnapshotSchedule) error {
	questdbsnapshotschedulelog.Info("Defaulting for QuestDBSnapshotSchedule", "name", obj.GetName())

	// TODO(user): fill in your defaulting logic.

	return nil
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
// NOTE: If you want to customise the 'path', use the flags '--defaulting-path' or '--validation-path'.
// +kubebuilder:webhook:path=/validate-crd-questdb-io-v1beta1-questdbsnapshotschedule,mutating=false,failurePolicy=fail,sideEffects=None,groups=crd.questdb.io,resources=questdbsnapshotschedules,verbs=create;update,versions=v1beta1,name=vquestdbsnapshotschedule-v1beta1.kb.io,admissionReviewVersions=v1

// QuestDBSnapshotScheduleCustomValidator struct is responsible for validating the QuestDBSnapshotSchedule resource
// when it is created, updated, or deleted.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as this struct is used only for temporary operations and does not need to be deeply copied.
type QuestDBSnapshotScheduleCustomValidator struct {
	// TODO(user): Add more fields as needed for validation
}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type QuestDBSnapshotSchedule.
func (v *QuestDBSnapshotScheduleCustomValidator) ValidateCreate(_ context.Context, obj *crdv1beta1.QuestDBSnapshotSchedule) (admission.Warnings, error) {
	questdbsnapshotschedulelog.Info("Validation for QuestDBSnapshotSchedule upon creation", "name", obj.GetName())

	// TODO(user): fill in your validation logic upon object creation.

	return nil, nil
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type QuestDBSnapshotSchedule.
func (v *QuestDBSnapshotScheduleCustomValidator) ValidateUpdate(_ context.Context, oldObj, newObj *crdv1beta1.QuestDBSnapshotSchedule) (admission.Warnings, error) {
	questdbsnapshotschedulelog.Info("Validation for QuestDBSnapshotSchedule upon update", "name", newObj.GetName())

	// TODO(user): fill in your validation logic upon object update.

	return nil, nil
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type QuestDBSnapshotSchedule.
func (v *QuestDBSnapshotScheduleCustomValidator) ValidateDelete(_ context.Context, obj *crdv1beta1.QuestDBSnapshotSchedule) (admission.Warnings, error) {
	questdbsnapshotschedulelog.Info("Validation for QuestDBSnapshotSchedule upon deletion", "name", obj.GetName())

	// TODO(user): fill in your validation logic upon object deletion.

	return nil, nil
}
