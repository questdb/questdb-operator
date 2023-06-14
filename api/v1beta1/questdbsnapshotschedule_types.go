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

// QuestDBSnapshotScheduleSpec defines the desired state of QuestDBSnapshotSchedule
type QuestDBSnapshotScheduleSpec struct {
	Schedule  string              `json:"schedule"`
	Retention int32               `json:"retention"`
	Snapshot  QuestDBSnapshotSpec `json:"snapshot"`
}

// QuestDBSnapshotStatus defines the observed state of QuestDBSnapshot
type QuestDBSnapshotScheduleStatus struct {
	NextSnapshot  metav1.Time          `json:"nextSnapshot,omitempty"`
	SnapshotPhase QuestDBSnapshotPhase `json:"snapshotPhase,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:shortName=qdbsched;qdbscheds
//+kubebuilder:printcolumn:name="Last Snapshot",type=string,JSONPath=`.status.snapshotPhase`

// QuestDBSnapshotSchedule is the Schema for the snapshotschedules API
type QuestDBSnapshotSchedule struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   QuestDBSnapshotScheduleSpec   `json:"spec,omitempty"`
	Status QuestDBSnapshotScheduleStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// QuestDBSnapshotScheduleList contains a list of QuestDBSnapshotSchedule
type QuestDBSnapshotScheduleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []QuestDBSnapshotSchedule `json:"items"`
}

func init() {
	SchemeBuilder.Register(&QuestDBSnapshotSchedule{}, &QuestDBSnapshotScheduleList{})
}
