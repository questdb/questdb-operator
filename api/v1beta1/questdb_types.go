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
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	AnnotationQuestDBName       = "questdb.crd.questdb.io/name"
	AnnotationQuestDBSecretType = "questdb.crd.questdb.io/secret-type"
)

type QuestDBResourcesSpec struct {
	Limits   v1.ResourceList `json:"limits,omitempty"`
	Requests v1.ResourceList `json:"requests,omitempty"`
}

type QuestDBVolumeSpec struct {
	Selector         *metav1.LabelSelector `json:"selector,omitempty"`
	Size             resource.Quantity     `json:"size,omitempty"`
	VolumeName       string                `json:"volumeName,omitempty"`
	StorageClassName *string               `json:"storageClassName,omitempty"`
	SnapshotName     string                `json:"snapshotName,omitempty"`
}

type QuestDBConfigSpec struct {
	ServerConfig string `json:"serverConfig,omitempty"`
	LogConfig    string `json:"logConfig,omitempty"`
}

// QuestDBSpec defines the desired state of QuestDB
type QuestDBSpec struct {
	Volume QuestDBVolumeSpec `json:"volume"`
	Config QuestDBConfigSpec `json:"config,omitempty"`

	Image string `json:"image"`

	Affinity          *v1.Affinity     `json:"affinity,omitempty"`
	ExtraEnv          []v1.EnvVar      `json:"extraEnv,omitempty"`
	ExtraVolumeMounts []v1.VolumeMount `json:"extraVolumeMounts,omitempty"`
	ExtraVolumes      []v1.Volume      `json:"extraVolumes,omitempty"`
	// ImagePullPolicy defaults to IfNotPresent
	ImagePullPolicy        v1.PullPolicy             `json:"imagePullPolicy,omitempty"`
	ImagePullSecrets       []v1.LocalObjectReference `json:"imagePullSecrets,omitempty"`
	NodeSelector           map[string]string         `json:"nodeSelector,omitempty"`
	PodAnnotations         map[string]string         `json:"podAnnotations,omitempty"`
	Resources              QuestDBResourcesSpec      `json:"resources,omitempty"`
	StatefulSetAnnotations map[string]string         `json:"statefulSetAnnotations,omitempty"`
	Tolerations            []v1.Toleration           `json:"tolerations,omitempty"`
}

type QuestDBEndpointStatus struct {
	Ilp  string `json:"ilp,omitempty"`
	Psql string `json:"psql,omitempty"`
	Http string `json:"http,omitempty"`
}

// QuestDBStatus defines the observed state of QuestDB
type QuestDBStatus struct {
	StatefulSetReadyReplicas int `json:"statefulSetReadyReplicas,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:shortName=qdb;qdbs
//+kubebuilder:printcolumn:name="Ready Replicas",type=integer,JSONPath=`.status.statefulSetReadyReplicas`
//+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// QuestDB is the Schema for the questdbs API
type QuestDB struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   QuestDBSpec   `json:"spec,omitempty"`
	Status QuestDBStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// QuestDBList contains a list of QuestDB
type QuestDBList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []QuestDB `json:"items"`
}

func init() {
	SchemeBuilder.Register(&QuestDB{}, &QuestDBList{})
}
