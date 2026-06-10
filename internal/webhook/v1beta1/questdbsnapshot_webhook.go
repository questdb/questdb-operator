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
var questdbsnapshotlog = logf.Log.WithName("questdbsnapshot-resource")

// SetupQuestDBSnapshotWebhookWithManager registers the webhook for QuestDBSnapshot in the manager.
func SetupQuestDBSnapshotWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &crdv1beta1.QuestDBSnapshot{}).
		WithValidator(&QuestDBSnapshotCustomValidator{}).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
// NOTE: If you want to customise the 'path', use the flags '--defaulting-path' or '--validation-path'.
// +kubebuilder:webhook:path=/validate-crd-questdb-io-v1beta1-questdbsnapshot,mutating=false,failurePolicy=fail,sideEffects=None,groups=crd.questdb.io,resources=questdbsnapshots,verbs=create;update,versions=v1beta1,name=vquestdbsnapshot-v1beta1.kb.io,admissionReviewVersions=v1

// QuestDBSnapshotCustomValidator struct is responsible for validating the QuestDBSnapshot resource
// when it is created, updated, or deleted.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as this struct is used only for temporary operations and does not need to be deeply copied.
type QuestDBSnapshotCustomValidator struct {
	// TODO(user): Add more fields as needed for validation
}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type QuestDBSnapshot.
func (v *QuestDBSnapshotCustomValidator) ValidateCreate(_ context.Context, obj *crdv1beta1.QuestDBSnapshot) (admission.Warnings, error) {
	questdbsnapshotlog.Info("Validation for QuestDBSnapshot upon creation", "name", obj.GetName())

	// TODO(user): fill in your validation logic upon object creation.

	return nil, nil
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type QuestDBSnapshot.
func (v *QuestDBSnapshotCustomValidator) ValidateUpdate(_ context.Context, oldObj, newObj *crdv1beta1.QuestDBSnapshot) (admission.Warnings, error) {
	questdbsnapshotlog.Info("Validation for QuestDBSnapshot upon update", "name", newObj.GetName())

	// TODO(user): fill in your validation logic upon object update.

	return nil, nil
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type QuestDBSnapshot.
func (v *QuestDBSnapshotCustomValidator) ValidateDelete(_ context.Context, obj *crdv1beta1.QuestDBSnapshot) (admission.Warnings, error) {
	questdbsnapshotlog.Info("Validation for QuestDBSnapshot upon deletion", "name", obj.GetName())

	// TODO(user): fill in your validation logic upon object deletion.

	return nil, nil
}
