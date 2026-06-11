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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// QuestDBBackupMethod selects how a backup is taken.
// +kubebuilder:validation:Enum=VolumeSnapshot
type QuestDBBackupMethod string

const (
	// BackupMethodVolumeSnapshot backs up the data volume via a CSI VolumeSnapshot,
	// bracketed by a QuestDB checkpoint for on-disk consistency. This is the only OSS method.
	BackupMethodVolumeSnapshot QuestDBBackupMethod = "VolumeSnapshot"
	// Reserved for QuestDB Enterprise (object-store backup) — not implemented:
	//   BackupMethodObjectStore QuestDBBackupMethod = "ObjectStore"
)

// QuestDBBackupPhase is the lifecycle phase of a backup.
type QuestDBBackupPhase string

const (
	// BackupPhasePending means the backup has been accepted but not yet started.
	BackupPhasePending QuestDBBackupPhase = "Pending"
	// BackupPhaseCheckpointCreated means CHECKPOINT CREATE succeeded; the DB is checkpoint-locked.
	BackupPhaseCheckpointCreated QuestDBBackupPhase = "CheckpointCreated"
	// BackupPhaseSnapshotCreated means the VolumeSnapshot is ready to use.
	BackupPhaseSnapshotCreated QuestDBBackupPhase = "SnapshotCreated"
	// BackupPhaseSucceeded is the terminal success phase (checkpoint released, snapshot ready).
	BackupPhaseSucceeded QuestDBBackupPhase = "Succeeded"
	// BackupPhaseFailed is the terminal failure phase.
	BackupPhaseFailed QuestDBBackupPhase = "Failed"
)

const (
	// BackupFinalizer guarantees CHECKPOINT RELEASE is attempted before a backup is deleted,
	// so a backup never strands an open checkpoint on the database.
	BackupFinalizer = "questdbbackup.crd.questdb.io/release-checkpoint"
)

// QuestDBBackupSpec defines the desired state of QuestDBBackup.
type QuestDBBackupSpec struct {
	// QuestDBName is the name of the QuestDB to back up (same namespace). Immutable.
	// +required
	QuestDBName string `json:"questdbName"`

	// Method selects the backup mechanism. Only VolumeSnapshot is supported (OSS).
	// +kubebuilder:default=VolumeSnapshot
	// +optional
	Method QuestDBBackupMethod `json:"method,omitempty"`

	// VolumeSnapshotClassName is the VolumeSnapshotClass used to snapshot the data volume.
	// If nil, the cluster's default VolumeSnapshotClass is used.
	// +optional
	VolumeSnapshotClassName *string `json:"volumeSnapshotClassName,omitempty"`
}

// QuestDBBackupStatus defines the observed state of QuestDBBackup.
type QuestDBBackupStatus struct {
	// Phase is the current lifecycle phase of the backup.
	// +optional
	Phase QuestDBBackupPhase `json:"phase,omitempty"`

	// Conditions represent the latest available observations of the backup's state.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// VolumeSnapshotName is the name of the created VolumeSnapshot.
	// +optional
	VolumeSnapshotName string `json:"volumeSnapshotName,omitempty"`

	// CheckpointCreatedAt is when CHECKPOINT CREATE succeeded.
	// +optional
	CheckpointCreatedAt *metav1.Time `json:"checkpointCreatedAt,omitempty"`

	// CheckpointReleasedAt is when CHECKPOINT RELEASE succeeded.
	// +optional
	CheckpointReleasedAt *metav1.Time `json:"checkpointReleasedAt,omitempty"`
}

// IsComplete reports whether the backup has reached a terminal phase.
func (b *QuestDBBackup) IsComplete() bool {
	return b.Status.Phase == BackupPhaseSucceeded || b.Status.Phase == BackupPhaseFailed
}

// CheckpointOutstanding reports whether a checkpoint was created but not yet released,
// i.e. CHECKPOINT RELEASE still must be attempted.
func (b *QuestDBBackup) CheckpointOutstanding() bool {
	return b.Status.CheckpointCreatedAt != nil && b.Status.CheckpointReleasedAt == nil
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=qdbbackup;qdbbackups
// +kubebuilder:printcolumn:name="QuestDB",type=string,JSONPath=`.spec.questdbName`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Snapshot",type=string,JSONPath=`.status.volumeSnapshotName`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// QuestDBBackup is the Schema for the questdbbackups API.
type QuestDBBackup struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec defines the desired state of QuestDBBackup
	// +required
	Spec QuestDBBackupSpec `json:"spec"`

	// status defines the observed state of QuestDBBackup
	// +optional
	Status QuestDBBackupStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// QuestDBBackupList contains a list of QuestDBBackup.
type QuestDBBackupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []QuestDBBackup `json:"items"`
}

func init() {
	SchemeBuilder.Register(&QuestDBBackup{}, &QuestDBBackupList{})
}
