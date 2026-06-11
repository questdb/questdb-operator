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

package controller

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"maps"
	"sort"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	crdv1beta2 "github.com/questdb/questdb-operator/api/v1beta2"
)

// orderedConfigKeys is the fixed render order of config files mounted into conf/, so the
// StatefulSet pod template is deterministic across reconciles.
var orderedConfigKeys = []string{"server.conf", "log.conf"}

// questdbImage returns the effective container image for a QuestDB.
func questdbImage(qdb *crdv1beta2.QuestDB) string {
	if qdb.Spec.Image != "" {
		return qdb.Spec.Image
	}
	return crdv1beta2.DefaultImage
}

// labelsForQuestDB returns the selector/identity labels for a QuestDB's child objects.
func labelsForQuestDB(name string) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       "questdb",
		"app.kubernetes.io/instance":   name,
		"app.kubernetes.io/managed-by": "questdb-operator",
	}
}

func mergeMaps(ms ...map[string]string) map[string]string {
	out := map[string]string{}
	for _, m := range ms {
		maps.Copy(out, m)
	}
	return out
}

// desiredConfigMap builds the ConfigMap holding QuestDB config files, or nil when none is configured.
func (r *QuestDBReconciler) desiredConfigMap(qdb *crdv1beta2.QuestDB) (*corev1.ConfigMap, error) {
	data := map[string]string{}
	if qdb.Spec.Config.ServerConfig != "" {
		data["server.conf"] = qdb.Spec.Config.ServerConfig
	}
	if qdb.Spec.Config.LogConfig != "" {
		data["log.conf"] = qdb.Spec.Config.LogConfig
	}
	if len(data) == 0 {
		return nil, nil
	}
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      qdb.Name,
			Namespace: qdb.Namespace,
			Labels:    labelsForQuestDB(qdb.Name),
		},
		Data: data,
	}
	if err := controllerutil.SetControllerReference(qdb, cm, r.Scheme); err != nil {
		return nil, err
	}
	return cm, nil
}

// hashConfigMap returns a stable hash of the ConfigMap data (empty string for nil).
func hashConfigMap(cm *corev1.ConfigMap) string {
	if cm == nil {
		return ""
	}
	keys := make([]string, 0, len(cm.Data))
	for k := range cm.Data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	h := sha256.New()
	for _, k := range keys {
		h.Write([]byte(k))
		h.Write([]byte{0})
		h.Write([]byte(cm.Data[k]))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

// desiredService builds the Service exposing QuestDB's ports.
func (r *QuestDBReconciler) desiredService(qdb *crdv1beta2.QuestDB) (*corev1.Service, error) {
	svcType := qdb.Spec.Service.Type
	if svcType == "" {
		svcType = corev1.ServiceTypeClusterIP
	}
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        qdb.Name,
			Namespace:   qdb.Namespace,
			Labels:      labelsForQuestDB(qdb.Name),
			Annotations: qdb.Spec.Service.Annotations,
		},
		Spec: corev1.ServiceSpec{
			Type:     svcType,
			Selector: labelsForQuestDB(qdb.Name),
			Ports: []corev1.ServicePort{
				{Name: "http", Port: portHTTP, TargetPort: intstr.FromInt(portHTTP)},
				{Name: "psql", Port: portPgWire, TargetPort: intstr.FromInt(portPgWire)},
				{Name: "ilp", Port: portILP, TargetPort: intstr.FromInt(portILP)},
				{Name: "metrics", Port: portHealth, TargetPort: intstr.FromInt(portHealth)},
			},
		},
	}
	if err := controllerutil.SetControllerReference(qdb, svc, r.Scheme); err != nil {
		return nil, err
	}
	return svc, nil
}

// desiredPVC builds the data PersistentVolumeClaim, optionally restored from a VolumeSnapshot.
func (r *QuestDBReconciler) desiredPVC(qdb *crdv1beta2.QuestDB) (*corev1.PersistentVolumeClaim, error) {
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      qdb.Name,
			Namespace: qdb.Namespace,
			Labels:    labelsForQuestDB(qdb.Name),
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceStorage: qdb.Spec.Volume.Size},
			},
			StorageClassName: qdb.Spec.Volume.StorageClassName,
			Selector:         qdb.Spec.Volume.Selector,
		},
	}
	if qdb.Spec.Volume.VolumeName != "" {
		pvc.Spec.VolumeName = qdb.Spec.Volume.VolumeName
	}
	if qdb.Spec.Volume.SnapshotName != "" {
		pvc.Spec.DataSource = &corev1.TypedLocalObjectReference{
			APIGroup: ptr.To("snapshot.storage.k8s.io"),
			Kind:     "VolumeSnapshot",
			Name:     qdb.Spec.Volume.SnapshotName,
		}
	}
	if err := controllerutil.SetControllerReference(qdb, pvc, r.Scheme); err != nil {
		return nil, err
	}
	return pvc, nil
}

// desiredStatefulSet builds the single-replica StatefulSet running QuestDB.
func (r *QuestDBReconciler) desiredStatefulSet(qdb *crdv1beta2.QuestDB, cm *corev1.ConfigMap, credSecretName, credHash string) (*appsv1.StatefulSet, error) {
	labels := labelsForQuestDB(qdb.Name)

	volumeMounts := []corev1.VolumeMount{{Name: "data", MountPath: dataDir}}
	volumes := []corev1.Volume{{
		Name:         "data",
		VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: qdb.Name}},
	}}

	if cm != nil {
		for _, key := range orderedConfigKeys {
			if _, ok := cm.Data[key]; ok {
				volumeMounts = append(volumeMounts, corev1.VolumeMount{
					Name:      "config",
					MountPath: confDir + "/" + key,
					SubPath:   key,
				})
			}
		}
		volumes = append(volumes, corev1.Volume{
			Name:         "config",
			VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: qdb.Name}}},
		})
	}
	volumeMounts = append(volumeMounts, qdb.Spec.ExtraVolumeMounts...)
	volumes = append(volumes, qdb.Spec.ExtraVolumes...)

	healthProbe := &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{Path: "/", Port: intstr.FromInt(portHealth)},
		},
		InitialDelaySeconds: 5,
		PeriodSeconds:       10,
		TimeoutSeconds:      3,
		FailureThreshold:    6,
	}

	container := corev1.Container{
		Name:            containerName,
		Image:           questdbImage(qdb),
		ImagePullPolicy: qdb.Spec.ImagePullPolicy,
		Ports: []corev1.ContainerPort{
			{Name: "http", ContainerPort: portHTTP},
			{Name: "psql", ContainerPort: portPgWire},
			{Name: "ilp", ContainerPort: portILP},
			{Name: "metrics", ContainerPort: portHealth},
		},
		EnvFrom:        []corev1.EnvFromSource{{SecretRef: &corev1.SecretEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: credSecretName}}}},
		Env:            qdb.Spec.ExtraEnv,
		Resources:      qdb.Spec.Resources,
		VolumeMounts:   volumeMounts,
		LivenessProbe:  healthProbe.DeepCopy(),
		ReadinessProbe: healthProbe.DeepCopy(),
	}

	var initContainers []corev1.Container
	if qdb.Spec.Volume.SnapshotName != "" {
		sentinel := restoreSentinelPrefix + qdb.Spec.Volume.SnapshotName
		initContainers = append(initContainers, corev1.Container{
			Name:    "restore-trigger",
			Image:   questdbImage(qdb),
			Command: []string{"sh", "-c", fmt.Sprintf("if [ ! -f %s ]; then touch %s && touch %s; fi", sentinel, restoreMarker, sentinel)},
			VolumeMounts: []corev1.VolumeMount{
				{Name: "data", MountPath: dataDir},
			},
		})
	}

	podAnnotations := mergeMaps(qdb.Spec.PodAnnotations, map[string]string{
		CredentialsHashAnnotation: credHash,
	})
	if h := hashConfigMap(cm); h != "" {
		podAnnotations[ConfigHashAnnotation] = h
	}

	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:        qdb.Name,
			Namespace:   qdb.Namespace,
			Labels:      labels,
			Annotations: qdb.Spec.StatefulSetAnnotations,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas:    ptr.To(int32(1)),
			ServiceName: qdb.Name,
			Selector:    &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      mergeMaps(labels, qdb.Spec.PodLabels),
					Annotations: podAnnotations,
				},
				Spec: corev1.PodSpec{
					InitContainers:   initContainers,
					Containers:       []corev1.Container{container},
					Volumes:          volumes,
					Affinity:         qdb.Spec.Affinity,
					NodeSelector:     qdb.Spec.NodeSelector,
					Tolerations:      qdb.Spec.Tolerations,
					SecurityContext:  qdb.Spec.PodSecurityContext,
					ImagePullSecrets: qdb.Spec.ImagePullSecrets,
				},
			},
		},
	}
	if err := controllerutil.SetControllerReference(qdb, sts, r.Scheme); err != nil {
		return nil, err
	}
	return sts, nil
}
