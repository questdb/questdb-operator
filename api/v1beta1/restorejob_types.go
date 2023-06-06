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

type RestoreJobPhase string

const (
	// RestoreJobPending means the restore job has not started yet
	RestoreJobPending RestoreJobPhase = "Pending"
	// RestoreJobRunning means the restore job is running
	RestoreJobRunning RestoreJobPhase = "Running"
	// RestoreJobFailed means the restore job has failed
	RestoreJobFailed RestoreJobPhase = "Failed"
	// RestoreJobSucceeded means the restore job has succeeded
	RestoreJobSucceeded RestoreJobPhase = "Succeeded"
)

// RestoreJobSpec defines the desired state of RestoreJob
type RestoreJobSpec struct {
	Snapshot string `json:"snapshot"`
}

// RestoreJobStatus defines the observed state of RestoreJob
type RestoreJobStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// RestoreJob is the Schema for the restorejobs API
type RestoreJob struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RestoreJobSpec   `json:"spec,omitempty"`
	Status RestoreJobStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// RestoreJobList contains a list of RestoreJob
type RestoreJobList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RestoreJob `json:"items"`
}

func init() {
	SchemeBuilder.Register(&RestoreJob{}, &RestoreJobList{})
}
