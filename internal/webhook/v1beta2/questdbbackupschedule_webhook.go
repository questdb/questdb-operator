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

	"github.com/robfig/cron/v3"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	crdv1beta2 "github.com/questdb/questdb-operator/api/v1beta2"
)

// scheduleRetentionDefault is applied when spec.retention is unset.
const scheduleRetentionDefault int32 = 7

// scheduleCronParser parses standard 5-field cron expressions.
var scheduleCronParser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

// nolint:unused
var questdbbackupschedulelog = logf.Log.WithName("questdbbackupschedule-resource")

// SetupQuestDBBackupScheduleWebhookWithManager registers the webhook for QuestDBBackupSchedule in the manager.
func SetupQuestDBBackupScheduleWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &crdv1beta2.QuestDBBackupSchedule{}).
		WithValidator(&QuestDBBackupScheduleCustomValidator{}).
		WithDefaulter(&QuestDBBackupScheduleCustomDefaulter{}).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-crd-questdb-io-v1beta2-questdbbackupschedule,mutating=true,failurePolicy=fail,sideEffects=None,groups=crd.questdb.io,resources=questdbbackupschedules,verbs=create;update,versions=v1beta2,name=mquestdbbackupschedule-v1beta2.kb.io,admissionReviewVersions=v1

// QuestDBBackupScheduleCustomDefaulter sets default values on QuestDBBackupSchedule resources.
type QuestDBBackupScheduleCustomDefaulter struct{}

// Default applies defaults to a QuestDBBackupSchedule.
func (d *QuestDBBackupScheduleCustomDefaulter) Default(_ context.Context, obj *crdv1beta2.QuestDBBackupSchedule) error {
	if obj.Spec.Retention == 0 {
		obj.Spec.Retention = scheduleRetentionDefault
	}
	if obj.Spec.Backup.Method == "" {
		obj.Spec.Backup.Method = crdv1beta2.BackupMethodVolumeSnapshot
	}
	return nil
}

// +kubebuilder:webhook:path=/validate-crd-questdb-io-v1beta2-questdbbackupschedule,mutating=false,failurePolicy=fail,sideEffects=None,groups=crd.questdb.io,resources=questdbbackupschedules,verbs=create;update,versions=v1beta2,name=vquestdbbackupschedule-v1beta2.kb.io,admissionReviewVersions=v1

// QuestDBBackupScheduleCustomValidator validates QuestDBBackupSchedule resources.
type QuestDBBackupScheduleCustomValidator struct{}

// ValidateCreate validates a QuestDBBackupSchedule on creation.
func (v *QuestDBBackupScheduleCustomValidator) ValidateCreate(_ context.Context, obj *crdv1beta2.QuestDBBackupSchedule) (admission.Warnings, error) {
	return nil, validateSchedule(obj)
}

// ValidateUpdate validates a QuestDBBackupSchedule on update.
func (v *QuestDBBackupScheduleCustomValidator) ValidateUpdate(_ context.Context, oldObj, newObj *crdv1beta2.QuestDBBackupSchedule) (admission.Warnings, error) {
	if newObj.Spec.Backup.QuestDBName != oldObj.Spec.Backup.QuestDBName {
		return nil, fmt.Errorf("spec.backup.questdbName is immutable")
	}
	return nil, validateSchedule(newObj)
}

// ValidateDelete is a no-op for QuestDBBackupSchedule.
func (v *QuestDBBackupScheduleCustomValidator) ValidateDelete(_ context.Context, _ *crdv1beta2.QuestDBBackupSchedule) (admission.Warnings, error) {
	return nil, nil
}

func validateSchedule(obj *crdv1beta2.QuestDBBackupSchedule) error {
	if _, err := scheduleCronParser.Parse(obj.Spec.Schedule); err != nil {
		return fmt.Errorf("spec.schedule is not a valid cron expression: %w", err)
	}
	if obj.Spec.Backup.QuestDBName == "" {
		return fmt.Errorf("spec.backup.questdbName is required")
	}
	return nil
}
