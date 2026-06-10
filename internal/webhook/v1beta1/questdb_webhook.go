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
var questdblog = logf.Log.WithName("questdb-resource")

// SetupQuestDBWebhookWithManager registers the webhook for QuestDB in the manager.
func SetupQuestDBWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &crdv1beta1.QuestDB{}).
		WithValidator(&QuestDBCustomValidator{}).
		WithDefaulter(&QuestDBCustomDefaulter{}).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

// +kubebuilder:webhook:path=/mutate-crd-questdb-io-v1beta1-questdb,mutating=true,failurePolicy=fail,sideEffects=None,groups=crd.questdb.io,resources=questdbs,verbs=create;update,versions=v1beta1,name=mquestdb-v1beta1.kb.io,admissionReviewVersions=v1

// QuestDBCustomDefaulter struct is responsible for setting default values on the custom resource of the
// Kind QuestDB when those are created or updated.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as it is used only for temporary operations and does not need to be deeply copied.
type QuestDBCustomDefaulter struct {
	// TODO(user): Add more fields as needed for defaulting
}

// Default implements webhook.CustomDefaulter so a webhook will be registered for the Kind QuestDB.
func (d *QuestDBCustomDefaulter) Default(_ context.Context, obj *crdv1beta1.QuestDB) error {
	questdblog.Info("Defaulting for QuestDB", "name", obj.GetName())

	// TODO(user): fill in your defaulting logic.

	return nil
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
// NOTE: If you want to customise the 'path', use the flags '--defaulting-path' or '--validation-path'.
// +kubebuilder:webhook:path=/validate-crd-questdb-io-v1beta1-questdb,mutating=false,failurePolicy=fail,sideEffects=None,groups=crd.questdb.io,resources=questdbs,verbs=create;update,versions=v1beta1,name=vquestdb-v1beta1.kb.io,admissionReviewVersions=v1

// QuestDBCustomValidator struct is responsible for validating the QuestDB resource
// when it is created, updated, or deleted.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as this struct is used only for temporary operations and does not need to be deeply copied.
type QuestDBCustomValidator struct {
	// TODO(user): Add more fields as needed for validation
}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type QuestDB.
func (v *QuestDBCustomValidator) ValidateCreate(_ context.Context, obj *crdv1beta1.QuestDB) (admission.Warnings, error) {
	questdblog.Info("Validation for QuestDB upon creation", "name", obj.GetName())

	// TODO(user): fill in your validation logic upon object creation.

	return nil, nil
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type QuestDB.
func (v *QuestDBCustomValidator) ValidateUpdate(_ context.Context, oldObj, newObj *crdv1beta1.QuestDB) (admission.Warnings, error) {
	questdblog.Info("Validation for QuestDB upon update", "name", newObj.GetName())

	// TODO(user): fill in your validation logic upon object update.

	return nil, nil
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type QuestDB.
func (v *QuestDBCustomValidator) ValidateDelete(_ context.Context, obj *crdv1beta1.QuestDB) (admission.Warnings, error) {
	questdblog.Info("Validation for QuestDB upon deletion", "name", obj.GetName())

	// TODO(user): fill in your validation logic upon object deletion.

	return nil, nil
}
