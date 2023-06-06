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

type SnapshotPhase string

const (
	// SnapshotPending means the snapshot has not started yet
	SnapshotPending SnapshotPhase = "Pending"
	// SnapshotRunning means the snapshot is running
	SnapshotRunning SnapshotPhase = "Running"
	// SnapshotFailed means the snapshot has failed
	SnapshotFailed SnapshotPhase = "Failed"
	// SnapshotSucceeded means the snapshot has succeeded
	SnapshotSucceeded SnapshotPhase = "Succeeded"
)

// SnapshotSpec defines the desired state of Snapshot
type SnapshotSpec struct {
	QuestDB  string                              `json:"questDB,omitempty"`
	Snapshot volumesnapshotv1.VolumeSnapshotSpec `json:"snapshot,omitempty"`
}

type SnapshotStatusS3 struct {
	Bucket string `json:"bucket,omitempty"`
	Key    string `json:"key,omitempty"`
}

type SnapshotStatusEBS struct {
	VolumeID string `json:"volumeID,omitempty"`
}

// SnapshotStatus defines the observed state of Snapshot
type SnapshotStatus struct {
	Phase SnapshotPhase     `json:"phase,omitempty"`
	S3    SnapshotStatusS3  `json:"s3,omitempty"`
	EBS   SnapshotStatusEBS `json:"ebs,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Snapshot is the Schema for the snapshots API
type Snapshot struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SnapshotSpec   `json:"spec,omitempty"`
	Status SnapshotStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// SnapshotList contains a list of Snapshot
type SnapshotList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Snapshot `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Snapshot{}, &SnapshotList{})
}
