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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// QuestDBSpec defines the desired state of a single-node QuestDB instance.
type QuestDBSpec struct {
	// Image is the QuestDB container image (including tag). If empty, the operator's
	// default OSS image is used (set by the defaulting webhook).
	// +optional
	Image string `json:"image,omitempty"`

	// ImagePullPolicy for the QuestDB container. Defaults to IfNotPresent.
	// +optional
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty"`

	// ImagePullSecrets are references to secrets used for pulling the QuestDB image.
	// +optional
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`

	// Volume is the persistent data volume configuration.
	// +required
	Volume QuestDBVolumeSpec `json:"volume"`

	// Config holds QuestDB server and log configuration.
	// +optional
	Config QuestDBConfigSpec `json:"config,omitempty"`

	// Auth configures QuestDB credentials. When omitted, the operator generates a
	// pg-wire user/password Secret automatically.
	// +optional
	Auth QuestDBAuthSpec `json:"auth,omitempty"`

	// Service configures the Service that exposes QuestDB on the network.
	// +optional
	Service QuestDBServiceSpec `json:"service,omitempty"`

	// Resources describes the compute resources for the QuestDB container.
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// Affinity for the QuestDB pod.
	// +optional
	Affinity *corev1.Affinity `json:"affinity,omitempty"`

	// NodeSelector for the QuestDB pod.
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// Tolerations for the QuestDB pod.
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// PodSecurityContext for the QuestDB pod. Defaults FSGroup so the data volume is writable.
	// +optional
	PodSecurityContext *corev1.PodSecurityContext `json:"podSecurityContext,omitempty"`

	// ExtraEnv are additional environment variables for the QuestDB container.
	// +optional
	ExtraEnv []corev1.EnvVar `json:"extraEnv,omitempty"`

	// ExtraVolumes are additional volumes added to the pod.
	// +optional
	ExtraVolumes []corev1.Volume `json:"extraVolumes,omitempty"`

	// ExtraVolumeMounts are additional volume mounts added to the QuestDB container.
	// +optional
	ExtraVolumeMounts []corev1.VolumeMount `json:"extraVolumeMounts,omitempty"`

	// PodAnnotations are added to the QuestDB pod template.
	// +optional
	PodAnnotations map[string]string `json:"podAnnotations,omitempty"`

	// PodLabels are added to the QuestDB pod template.
	// +optional
	PodLabels map[string]string `json:"podLabels,omitempty"`

	// StatefulSetAnnotations are added to the managed StatefulSet object.
	// +optional
	StatefulSetAnnotations map[string]string `json:"statefulSetAnnotations,omitempty"`
}

// QuestDBVolumeSpec configures the persistent data volume.
type QuestDBVolumeSpec struct {
	// Size is the requested size of the data volume. May be increased (expansion) but not shrunk.
	// +required
	Size resource.Quantity `json:"size"`

	// StorageClassName for the data volume. If nil, the cluster default is used.
	// +optional
	StorageClassName *string `json:"storageClassName,omitempty"`

	// Selector binds the data PVC to a PersistentVolume matching this selector.
	// +optional
	Selector *metav1.LabelSelector `json:"selector,omitempty"`

	// VolumeName binds the data PVC to a specific PersistentVolume by name.
	// +optional
	VolumeName string `json:"volumeName,omitempty"`

	// SnapshotName restores the data volume from the named VolumeSnapshot on creation.
	// The operator triggers QuestDB checkpoint recovery on first boot. Immutable.
	// +optional
	SnapshotName string `json:"snapshotName,omitempty"`
}

// QuestDBConfigSpec holds QuestDB configuration files.
type QuestDBConfigSpec struct {
	// ServerConfig is the content of QuestDB's server.conf. Operator-reserved keys are appended.
	// +optional
	ServerConfig string `json:"serverConfig,omitempty"`

	// LogConfig is the content of QuestDB's log.conf. A sensible default is used when empty.
	// +optional
	LogConfig string `json:"logConfig,omitempty"`
}

// QuestDBAuthSpec configures QuestDB authentication. OSS supports pg-wire credentials only;
// additional fields (HTTP/ILP token auth) are reserved for QuestDB Enterprise and not implemented.
type QuestDBAuthSpec struct {
	// Psql configures pg-wire (PostgreSQL wire protocol) credentials. When nil, the operator
	// generates a Secret "<name>-credentials" with a random password.
	// +optional
	Psql *QuestDBPsqlAuthSpec `json:"psql,omitempty"`
}

// QuestDBPsqlAuthSpec references a Secret holding pg-wire credentials.
type QuestDBPsqlAuthSpec struct {
	// SecretName references a Secret (same namespace) with keys QDB_PG_USER and QDB_PG_PASSWORD.
	// +required
	SecretName string `json:"secretName"`
}

// QuestDBServiceSpec configures network exposure of QuestDB.
type QuestDBServiceSpec struct {
	// Type of the Service. Defaults to ClusterIP.
	// +optional
	Type corev1.ServiceType `json:"type,omitempty"`

	// Annotations added to the Service (e.g. cloud load-balancer hints).
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}

// QuestDBStatus defines the observed state of QuestDB.
type QuestDBStatus struct {
	// Conditions represent the latest available observations of the QuestDB's state.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ReadyReplicas is the number of ready replicas of the underlying StatefulSet (0 or 1).
	// +optional
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`

	// CredentialsSecretName is the effective Secret holding pg-wire credentials (referenced or generated).
	// +optional
	CredentialsSecretName string `json:"credentialsSecretName,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=qdb
// +kubebuilder:printcolumn:name="Ready",type=integer,JSONPath=`.status.readyReplicas`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// QuestDB is the Schema for the questdbs API.
type QuestDB struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec defines the desired state of QuestDB
	// +required
	Spec QuestDBSpec `json:"spec"`

	// status defines the observed state of QuestDB
	// +optional
	Status QuestDBStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// QuestDBList contains a list of QuestDB.
type QuestDBList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []QuestDB `json:"items"`
}

func init() {
	SchemeBuilder.Register(&QuestDB{}, &QuestDBList{})
}
