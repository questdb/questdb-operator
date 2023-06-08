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
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var questdbsnapshotlog = logf.Log.WithName("questdbsnapshot-resource")

func (r *QuestDBSnapshot) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

//+kubebuilder:webhook:path=/mutate-crd-questdb-io-v1beta1-questdbsnapshot,mutating=true,failurePolicy=fail,sideEffects=None,groups=crd.questdb.io,resources=questdbsnapshots,verbs=create;update,versions=v1beta1,name=mquestdbsnapshot.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &QuestDBSnapshot{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *QuestDBSnapshot) Default() {
	questdbsnapshotlog.Info("default", "name", r.Name)

	// todo: add spec field and defaulting check for max job retries
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-crd-questdb-io-v1beta1-questdbsnapshot,mutating=false,failurePolicy=fail,sideEffects=None,groups=crd.questdb.io,resources=questdbsnapshots,verbs=create;update,versions=v1beta1,name=vquestdbsnapshot.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &QuestDBSnapshot{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *QuestDBSnapshot) ValidateCreate() error {
	questdbsnapshotlog.Info("validate create", "name", r.Name)

	// todo: Check that the questdb exists
	// todo: Check thast the snapshotclass exists

	// TODO(user): fill in your validation logic upon object creation.
	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *QuestDBSnapshot) ValidateUpdate(old runtime.Object) error {
	questdbsnapshotlog.Info("validate update", "name", r.Name)

	// TODO(user): fill in your validation logic upon object update.
	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *QuestDBSnapshot) ValidateDelete() error {
	questdbsnapshotlog.Info("validate delete", "name", r.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil
}
