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
	// SnapshotFinalizing means the snapshot is finalizing
	SnapshotFinalizing QuestDBSnapshotPhase = "Finalizing"
	// SnapshotFailed means the snapshot has failed
	SnapshotFailed QuestDBSnapshotPhase = "Failed"
	// SnapshotSucceeded means the snapshot has succeeded
	SnapshotSucceeded QuestDBSnapshotPhase = "Succeeded"
)

const (
	SnapshotCompleteFinalizer = "questdbsnapshot.crd.questdb.io/snapshot-complete-finalizer"
)

// QuestDBSnapshotSpec defines the desired state of QuestDBSnapshot
type QuestDBSnapshotSpec struct {
	QuestDBName             string  `json:"questdbName"`
	VolumeSnapshotClassName *string `json:"volumeSnapshotClassName,omitempty"`
	JobBackoffLimit         int32   `json:"jobBackoffLimit,omitempty"`
}

// QuestDBSnapshotStatus defines the observed state of QuestDBSnapshot
type QuestDBSnapshotStatus struct {
	Phase QuestDBSnapshotPhase `json:"phase,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:shortName=qdbsnap;qdbsnaps
//+kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
//+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// QuestDBSnapshot is the Schema for the snapshots API
type QuestDBSnapshot struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   QuestDBSnapshotSpec   `json:"spec,omitempty"`
	Status QuestDBSnapshotStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// QuestDBSnapshotList contains a list of QuestDBSnapshot
type QuestDBSnapshotList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []QuestDBSnapshot `json:"items"`
}

func init() {
	SchemeBuilder.Register(&QuestDBSnapshot{}, &QuestDBSnapshotList{})
}

func (s QuestDBSnapshot) IsComplete() bool {
	return s.Status.Phase == SnapshotSucceeded || s.Status.Phase == SnapshotFailed
}
