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
	"github.com/robfig/cron/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DefaultRetention is the number of most-recent successful backups kept when spec.retention is unset
// (zero). A negative retention disables pruning. Shared by the defaulting webhook and the controller
// so the two cannot drift.
const DefaultRetention int32 = 7

// ScheduleCronParser parses the standard 5-field cron expressions (minute hour dom month dow) accepted
// in spec.schedule, in UTC. Shared by the validating webhook and the controller so an expression the
// webhook admits is exactly one the controller can parse.
var ScheduleCronParser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

// QuestDBBackupScheduleSpec defines the desired state of QuestDBBackupSchedule.
type QuestDBBackupScheduleSpec struct {
	// Schedule is a standard 5-field cron expression (minute hour dom month dow) in UTC.
	// +required
	Schedule string `json:"schedule"`

	// Backup is the template used for each QuestDBBackup created by this schedule.
	// +required
	Backup QuestDBBackupSpec `json:"backup"`

	// Retention is the number of most-recent successful backups to keep; older successful
	// backups are garbage-collected. Defaults to 7 (applied when unset/zero). A negative value
	// disables pruning entirely, keeping all successful backups.
	// +optional
	Retention int32 `json:"retention,omitempty"`

	// Suspend pauses the creation of new backups when true. Existing backups are untouched.
	// +optional
	Suspend bool `json:"suspend,omitempty"`
}

// QuestDBBackupScheduleStatus defines the observed state of QuestDBBackupSchedule.
type QuestDBBackupScheduleStatus struct {
	// Conditions represent the latest available observations of the schedule's state.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// LastBackupTime is when this schedule last created a backup.
	// +optional
	LastBackupTime *metav1.Time `json:"lastBackupTime,omitempty"`

	// LastBackupName is the name of the most recently created backup.
	// +optional
	LastBackupName string `json:"lastBackupName,omitempty"`

	// LastBackupPhase mirrors the phase of the most recently created backup.
	// +optional
	LastBackupPhase QuestDBBackupPhase `json:"lastBackupPhase,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=qdbsched;qdbscheds
// +kubebuilder:printcolumn:name="Schedule",type=string,JSONPath=`.spec.schedule`
// +kubebuilder:printcolumn:name="QuestDB",type=string,JSONPath=`.spec.backup.questdbName`
// +kubebuilder:printcolumn:name="Last Backup",type=date,JSONPath=`.status.lastBackupTime`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// QuestDBBackupSchedule is the Schema for the questdbbackupschedules API.
type QuestDBBackupSchedule struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec defines the desired state of QuestDBBackupSchedule
	// +required
	Spec QuestDBBackupScheduleSpec `json:"spec"`

	// status defines the observed state of QuestDBBackupSchedule
	// +optional
	Status QuestDBBackupScheduleStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// QuestDBBackupScheduleList contains a list of QuestDBBackupSchedule.
type QuestDBBackupScheduleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []QuestDBBackupSchedule `json:"items"`
}

func init() {
	SchemeBuilder.Register(&QuestDBBackupSchedule{}, &QuestDBBackupScheduleList{})
}
