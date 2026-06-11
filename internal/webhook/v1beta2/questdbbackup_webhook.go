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

package v1beta2

import (
	"context"
	"fmt"
	"reflect"

	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	crdv1beta2 "github.com/questdb/questdb-operator/api/v1beta2"
)

// nolint:unused
var questdbbackuplog = logf.Log.WithName("questdbbackup-resource")

// SetupQuestDBBackupWebhookWithManager registers the webhook for QuestDBBackup in the manager.
func SetupQuestDBBackupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &crdv1beta2.QuestDBBackup{}).
		WithValidator(&QuestDBBackupCustomValidator{}).
		WithDefaulter(&QuestDBBackupCustomDefaulter{}).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-crd-questdb-io-v1beta2-questdbbackup,mutating=true,failurePolicy=fail,sideEffects=None,groups=crd.questdb.io,resources=questdbbackups,verbs=create;update,versions=v1beta2,name=mquestdbbackup-v1beta2.kb.io,admissionReviewVersions=v1

// QuestDBBackupCustomDefaulter sets default values on QuestDBBackup resources.
type QuestDBBackupCustomDefaulter struct{}

// Default applies defaults to a QuestDBBackup.
func (d *QuestDBBackupCustomDefaulter) Default(_ context.Context, obj *crdv1beta2.QuestDBBackup) error {
	if obj.Spec.Method == "" {
		obj.Spec.Method = crdv1beta2.BackupMethodVolumeSnapshot
	}
	return nil
}

// +kubebuilder:webhook:path=/validate-crd-questdb-io-v1beta2-questdbbackup,mutating=false,failurePolicy=fail,sideEffects=None,groups=crd.questdb.io,resources=questdbbackups,verbs=create;update,versions=v1beta2,name=vquestdbbackup-v1beta2.kb.io,admissionReviewVersions=v1

// QuestDBBackupCustomValidator validates QuestDBBackup resources.
type QuestDBBackupCustomValidator struct{}

// ValidateCreate validates a QuestDBBackup on creation.
func (v *QuestDBBackupCustomValidator) ValidateCreate(_ context.Context, obj *crdv1beta2.QuestDBBackup) (admission.Warnings, error) {
	if obj.Spec.QuestDBName == "" {
		return nil, fmt.Errorf("spec.questdbName is required")
	}
	return nil, nil
}

// ValidateUpdate enforces immutability of a QuestDBBackup's identity fields.
func (v *QuestDBBackupCustomValidator) ValidateUpdate(_ context.Context, oldObj, newObj *crdv1beta2.QuestDBBackup) (admission.Warnings, error) {
	if newObj.Spec.QuestDBName != oldObj.Spec.QuestDBName {
		return nil, fmt.Errorf("spec.questdbName is immutable")
	}
	if newObj.Spec.Method != oldObj.Spec.Method {
		return nil, fmt.Errorf("spec.method is immutable")
	}
	if !reflect.DeepEqual(newObj.Spec.VolumeSnapshotClassName, oldObj.Spec.VolumeSnapshotClassName) {
		return nil, fmt.Errorf("spec.volumeSnapshotClassName is immutable")
	}
	return nil, nil
}

// ValidateDelete is a no-op for QuestDBBackup.
func (v *QuestDBBackupCustomValidator) ValidateDelete(_ context.Context, _ *crdv1beta2.QuestDBBackup) (admission.Warnings, error) {
	return nil, nil
}
