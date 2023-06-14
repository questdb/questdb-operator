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
	"fmt"

	cron "github.com/robfig/cron/v3"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

const (
	ScheduleRetentionDefault int32 = 7
)

// log is for logging in this package.
var questdbsnapshotschedulelog = logf.Log.WithName("questdbsnapshotschedule-resource")

func (r *QuestDBSnapshotSchedule) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//+kubebuilder:webhook:path=/mutate-crd-questdb-io-v1beta1-questdbsnapshotschedule,mutating=true,failurePolicy=fail,sideEffects=None,groups=crd.questdb.io,resources=questdbsnapshotschedules,verbs=create;update,versions=v1beta1,name=mquestdbsnapshotschedule.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &QuestDBSnapshotSchedule{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *QuestDBSnapshotSchedule) Default() {
	questdbsnapshotschedulelog.Info("default", "name", r.Name)

	if r.Spec.Snapshot.JobBackoffLimit == 0 {
		r.Spec.Snapshot.JobBackoffLimit = JobBackoffLimitDefault
	}

	if r.Spec.Retention == 0 {
		r.Spec.Retention = ScheduleRetentionDefault
	}
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-crd-questdb-io-v1beta1-questdbsnapshotschedule,mutating=false,failurePolicy=fail,sideEffects=None,groups=crd.questdb.io,resources=questdbsnapshotschedules,verbs=create;update,versions=v1beta1,name=vquestdbsnapshotschedule.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &QuestDBSnapshotSchedule{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *QuestDBSnapshotSchedule) ValidateCreate() error {
	questdbsnapshotschedulelog.Info("validate create", "name", r.Name)

	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	_, err := parser.Parse(r.Spec.Schedule)
	if err != nil {
		return fmt.Errorf("invalid schedule: %w", err)
	}

	return validateSnapshotCreate(r.Spec.Snapshot)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *QuestDBSnapshotSchedule) ValidateUpdate(old runtime.Object) error {
	questdbsnapshotschedulelog.Info("validate update", "name", r.Name)

	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	_, err := parser.Parse(r.Spec.Schedule)
	if err != nil {
		return fmt.Errorf("invalid schedule: %w", err)
	}

	oldSched, valid := old.(*QuestDBSnapshotSchedule)
	if !valid {
		return errors.New("old object is not a QuestDBSnapshotSchedule")
	}

	return validateSnapshotUpdate(oldSched.Spec.Snapshot, r.Spec.Snapshot)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *QuestDBSnapshotSchedule) ValidateDelete() error {
	questdbsnapshotschedulelog.Info("validate delete", "name", r.Name)

	// Nothing to validate here
	return nil
}
