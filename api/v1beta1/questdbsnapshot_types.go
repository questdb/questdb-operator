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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type QuestDBSnapshotPhase string

const (
	// SnapshotPending means the snapshot has not started yet
	SnapshotPending QuestDBSnapshotPhase = "Pending"
	// SnapshotRunning means the snapshot is running
	SnapshotRunning QuestDBSnapshotPhase = "Running"
	// SnapshotCleaning means the snapshot is cleaning up
	SnapshotCleaning QuestDBSnapshotPhase = "Cleaning"
	// SnapshotFailed means the snapshot has failed
	SnapshotFailed QuestDBSnapshotPhase = "Failed"
	// SnapshotSucceeded means the snapshot has succeeded
	SnapshotSucceeded QuestDBSnapshotPhase = "Succeeded"
)

// SnapshotSpec defines the desired state of QuestDBSnapshot
type SnapshotSpec struct {
	QuestDB             string `json:"questdb"`
	VolumeSnapshotClass string `json:"volumeSnapshotClass"`
}

// SnapshotStatus defines the observed state of QuestDBSnapshot
type SnapshotStatus struct {
	Phase            QuestDBSnapshotPhase `json:"phase,omitempty"`
	SnapshotStarted  metav1.Time          `json:"snapshotStarted,omitempty"`
	SnapshotFinished metav1.Time          `json:"snapshotFinished,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// QuestDBSnapshot is the Schema for the snapshots API
type QuestDBSnapshot struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SnapshotSpec   `json:"spec,omitempty"`
	Status SnapshotStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// SnapshotList contains a list of QuestDBSnapshot
type SnapshotList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []QuestDBSnapshot `json:"items"`
}

func init() {
	SchemeBuilder.Register(&QuestDBSnapshot{}, &SnapshotList{})
}
