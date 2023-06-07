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
	volumesnapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SnapshotScheduleSpec defines the desired state of QuestDBSnapshotSchedule
type SnapshotScheduleSpec struct {
	Schedule           string                              `json:"schedule"`
	Snapshot           volumesnapshotv1.VolumeSnapshotSpec `json:"snapshot,omitempty"`
	ServiceAccountName string                              `json:"serviceAccountName"`
}

// SnapshotScheduleStatus defines the observed state of QuestDBSnapshotSchedule
type SnapshotScheduleStatus struct {
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// QuestDBSnapshotSchedule is the Schema for the snapshotschedules API
type QuestDBSnapshotSchedule struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SnapshotScheduleSpec   `json:"spec,omitempty"`
	Status SnapshotScheduleStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// SnapshotScheduleList contains a list of QuestDBSnapshotSchedule
type SnapshotScheduleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []QuestDBSnapshotSchedule `json:"items"`
}

func init() {
	SchemeBuilder.Register(&QuestDBSnapshotSchedule{}, &SnapshotScheduleList{})
}
