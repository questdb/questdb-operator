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

package controller

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	crdv1beta1 "github.com/questdb/questdb-operator/api/v1beta1"
	"github.com/questdb/questdb-operator/internal/secrets"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// QuestDBReconciler reconciles a QuestDB object
type QuestDBReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

//+kubebuilder:rbac:groups=crd.questdb.io,resources=questdbs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=crd.questdb.io,resources=questdbs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=crd.questdb.io,resources=questdbs/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=events,verbs=create;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *QuestDBReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	var (
		err error

		q = &crdv1beta1.QuestDB{}
		_ = log.FromContext(ctx)
	)

	// Try to get the object we are reconciling.  Exit if it does not exist
	if err = r.Get(ctx, req.NamespacedName, q); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Get its secrets
	secrets, err := secrets.GetSecrets(ctx, r.Client, q)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Reconcile the ConfigMap
	if err = r.reconcileConfigMap(ctx, q, secrets); err != nil {
		return ctrl.Result{}, err
	}

	// Reconcile the PVC
	if err = r.reconcilePvc(ctx, q); err != nil {
		return ctrl.Result{}, err
	}

	// Reconcile the StatefulSet
	if err = r.reconcileStatefulSet(ctx, q, secrets); err != nil {
		return ctrl.Result{}, err
	}

	// Reconcile the Service
	if err = r.reconcileService(ctx, q); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *QuestDBReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&crdv1beta1.QuestDB{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&v1.PersistentVolumeClaim{}).
		Owns(&v1.Service{}).
		Owns(&v1.ConfigMap{}).
		Complete(r)
}

func (r *QuestDBReconciler) buildStatefulSet(q *crdv1beta1.QuestDB, s secrets.QuestDBSecrets) appsv1.StatefulSet {

	sts := appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      q.Name,
			Namespace: q.Namespace,
			Labels:    q.Labels,
		},
		Spec: appsv1.StatefulSetSpec{
			ServiceName: q.Name,
			Replicas:    pointer.Int32(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: q.Labels,
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:      q.Name,
					Namespace: q.Namespace,
					Labels:    q.Labels,
				},
				Spec: v1.PodSpec{
					Affinity: q.Spec.Affinity,
					Containers: []v1.Container{
						{
							Name:  "questdb",
							Image: q.Spec.Image,
							//Image:           "busybox:latest",
							//Command:         []string{"sleep", "infinity"},
							ImagePullPolicy: q.Spec.ImagePullPolicy,
							Env:             q.Spec.ExtraEnv,
							Ports: []v1.ContainerPort{
								{
									Name:          "http",
									ContainerPort: 9000,
								},
								{
									Name:          "psql",
									ContainerPort: 8812,
								},
								{
									Name:          "ilp",
									ContainerPort: 9009,
								},
								{
									Name:          "metrics",
									ContainerPort: 9003,
								},
							},
							Resources: v1.ResourceRequirements{
								Limits:   q.Spec.Resources.Limits,
								Requests: q.Spec.Resources.Requests,
							},
							LivenessProbe: &v1.Probe{
								FailureThreshold:    5,
								InitialDelaySeconds: 5,
								PeriodSeconds:       10,
								TimeoutSeconds:      2,
								ProbeHandler: v1.ProbeHandler{
									HTTPGet: &v1.HTTPGetAction{
										Path:   "/status",
										Port:   intstr.FromInt(9003),
										Scheme: v1.URISchemeHTTP,
									},
								},
							},
							ReadinessProbe: &v1.Probe{
								FailureThreshold:    3,
								InitialDelaySeconds: 5,
								PeriodSeconds:       10,
								SuccessThreshold:    1,
								TimeoutSeconds:      2,
								ProbeHandler: v1.ProbeHandler{
									HTTPGet: &v1.HTTPGetAction{
										Path:   "/status",
										Port:   intstr.FromInt(9003),
										Scheme: v1.URISchemeHTTP,
									},
								},
							},
							VolumeMounts: []v1.VolumeMount{
								{
									Name:      "data",
									MountPath: "/var/lib/questdb/db",
								},
								{
									Name:      "config",
									MountPath: "/var/lib/questdb/conf",
								},
							},
						},
					},
					ImagePullSecrets: q.Spec.ImagePullSecrets,
					NodeSelector:     q.Spec.NodeSelector,
					SecurityContext: &v1.PodSecurityContext{
						FSGroup: pointer.Int64(10001),
					},
					Tolerations: q.Spec.Tolerations,
					Volumes: []v1.Volume{
						{
							Name: "data",
							VolumeSource: v1.VolumeSource{
								PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
									ClaimName: q.Name,
								},
							},
						},
						{
							Name: "config",
							VolumeSource: v1.VolumeSource{
								ConfigMap: &v1.ConfigMapVolumeSource{
									LocalObjectReference: v1.LocalObjectReference{
										Name: q.Name,
									},
								},
							},
						},
					},
				},
			},
		},
	}

	// Add ILP mount if needed
	if s.IlpSecret != nil {
		sts.Spec.Template.Spec.Containers[0].VolumeMounts = append(sts.Spec.Template.Spec.Containers[0].VolumeMounts, v1.VolumeMount{
			Name:      "ilp",
			MountPath: "/var/lib/questdb/auth",
		})

		sts.Spec.Template.Spec.Volumes = append(sts.Spec.Template.Spec.Volumes, v1.Volume{
			Name: "ilp",
			VolumeSource: v1.VolumeSource{
				Secret: &v1.SecretVolumeSource{
					SecretName: s.IlpSecret.Name,
				},
			},
		})
	}

	// Add PSQL env var mount if needed
	if s.PsqlSecret != nil {
		sts.Spec.Template.Spec.Containers[0].EnvFrom = append(sts.Spec.Template.Spec.Containers[0].EnvFrom, v1.EnvFromSource{
			SecretRef: &v1.SecretEnvSource{
				LocalObjectReference: v1.LocalObjectReference{
					Name: s.PsqlSecret.Name,
				},
			},
		})
	}

	if err := ctrl.SetControllerReference(q, &sts, r.Scheme); err != nil {
		panic(fmt.Sprintf("failed to set controller reference, even though we are building an object from scratch: %s", err.Error()))
	}
	return sts

}

func (r *QuestDBReconciler) reconcileStatefulSet(ctx context.Context, q *crdv1beta1.QuestDB, s secrets.QuestDBSecrets) error {
	var (
		err    error
		actual = &appsv1.StatefulSet{}
	)

	if err = r.Get(ctx, client.ObjectKeyFromObject(q), actual); err != nil {
		if client.IgnoreNotFound(err) != nil {
			return err
		}

		desired := r.buildStatefulSet(q, s)

		if err = r.Create(ctx, &desired); err != nil {
			r.Recorder.Event(q, v1.EventTypeWarning, "StatefulSetCreateFailed", err.Error())
			return err
		}

		r.Recorder.Event(q, v1.EventTypeNormal, "StatefulSetCreated", "StatefulSet created")

		*actual = desired
	}

	// Update the StatefulSet image if needed
	if actual.Spec.Template.Spec.Containers[0].Image != q.Spec.Image {
		actual.Spec.Template.Spec.Containers[0].Image = q.Spec.Image
		if err = r.Update(ctx, actual); err != nil {
			r.Recorder.Event(q, v1.EventTypeWarning, "StatefulSetUpdateFailed", err.Error())
			return err
		}

		r.Recorder.Event(q, v1.EventTypeNormal, "StatefulSetUpdated", "StatefulSet updated")
	}

	return nil

}

func (r *QuestDBReconciler) buildService(q *crdv1beta1.QuestDB) v1.Service {
	svc := v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      q.Name,
			Namespace: q.Namespace,
			Labels:    q.Labels,
		},
		Spec: v1.ServiceSpec{
			Ports: []v1.ServicePort{
				{
					Name: "http",
					Port: 9000,
				},
				{
					Name: "psql",
					Port: 8812,
				},
				{
					Name: "ilp",
					Port: 9009,
				},
				{
					Name: "metrics",
					Port: 9003,
				},
			},
			Selector: q.ObjectMeta.Labels,
		},
	}

	if err := ctrl.SetControllerReference(q, &svc, r.Scheme); err != nil {
		panic(fmt.Sprintf("failed to set controller reference, even though we are building an object from scratch: %s", err.Error()))
	}

	return svc
}

func (r *QuestDBReconciler) reconcileService(ctx context.Context, q *crdv1beta1.QuestDB) error {
	var (
		err error

		actual  = &v1.Service{}
		desired = r.buildService(q)
	)
	if err = r.Get(ctx, client.ObjectKeyFromObject(q), actual); err != nil {
		if client.IgnoreNotFound(err) != nil {
			return err
		}

		if err = r.Create(ctx, &desired); err != nil {
			return err
		}

		r.Recorder.Event(q, v1.EventTypeNormal, "ServiceCreated", "Service created")

		*actual = desired
	}

	// Update the Service ports if needed
	if !reflect.DeepEqual(actual.Spec.Ports, q.Spec.Ports) {
		actual.Spec.Ports = desired.Spec.Ports
		if err = r.Update(ctx, actual); err != nil {
			return err
		}

		r.Recorder.Event(q, v1.EventTypeNormal, "ServiceUpdated", "Service updated")
	}

	return nil
}

func (r *QuestDBReconciler) buildPvc(q *crdv1beta1.QuestDB) v1.PersistentVolumeClaim {

	pvc := v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      q.Name,
			Namespace: q.Namespace,
			Labels:    q.Labels,
		},
		Spec: v1.PersistentVolumeClaimSpec{
			AccessModes: []v1.PersistentVolumeAccessMode{
				v1.ReadWriteOnce,
			},
		},
	}

	if q.Spec.Volume.StorageClassName != nil {
		pvc.Spec.StorageClassName = q.Spec.Volume.StorageClassName
	}

	if q.Spec.Volume.VolumeName != "" {
		pvc.Spec.VolumeName = q.Spec.Volume.VolumeName
	}

	if q.Spec.Volume.Selector != nil {
		pvc.Spec.Selector = q.Spec.Volume.Selector
	}

	pvc.Spec.Resources.Requests = v1.ResourceList{
		v1.ResourceStorage: q.Spec.Volume.Size,
	}

	if q.Spec.Volume.SnapshotName != "" {
		pvc.Spec.DataSource = &v1.TypedLocalObjectReference{
			APIGroup: pointer.String("snapshot.storage.k8s.io"),
			Kind:     "VolumeSnapshot",
			Name:     q.Spec.Volume.SnapshotName,
		}
	}

	if err := ctrl.SetControllerReference(q, &pvc, r.Scheme); err != nil {
		panic(fmt.Sprintf("failed to set controller reference, even though we are building an object from scratch: %s", err.Error()))
	}

	return pvc

}

func (r *QuestDBReconciler) reconcilePvc(ctx context.Context, q *crdv1beta1.QuestDB) error {
	var (
		err    error
		actual = &v1.PersistentVolumeClaim{}
	)

	if err = r.Get(ctx, client.ObjectKeyFromObject(q), actual); err != nil {
		if client.IgnoreNotFound(err) != nil {
			return err
		}
		desired := r.buildPvc(q)
		if err = r.Create(ctx, &desired); err != nil {
			r.Recorder.Event(q, v1.EventTypeWarning, "PVCCreateFailed", err.Error())
			return err
		}
		r.Recorder.Event(q, v1.EventTypeNormal, "PVCCreated", "PVC created")

		*actual = desired
	}

	// Resize the PVC if needed
	if actual.Spec.Resources.Requests[v1.ResourceStorage] != q.Spec.Volume.Size {
		actual.Spec.Resources.Requests = v1.ResourceList{
			v1.ResourceStorage: q.Spec.Volume.Size,
		}
		if err = r.Update(ctx, actual); err != nil {
			r.Recorder.Event(q, v1.EventTypeWarning, "PVCResizeFailed", err.Error())
			return err
		}
		r.Recorder.Event(q, v1.EventTypeNormal, "PVCResized", "PVC resized")
	}

	return nil
}

func buildDbConfigSuffix(q *crdv1beta1.QuestDB, secrets secrets.QuestDBSecrets) string {
	dbConfig := strings.Builder{}
	dbConfig.WriteRune('\n')
	dbConfig.WriteString("### Reserved values -- set by the operator ###\n")

	if secrets.IlpSecret != nil {
		dbConfig.WriteString(fmt.Sprintf("ilp.auth.file=%s\n", "/var/lib/questdb/auth/auth.json"))
	}

	return dbConfig.String()
}

func (r *QuestDBReconciler) buildConfigMap(q *crdv1beta1.QuestDB, s secrets.QuestDBSecrets) v1.ConfigMap {
	logConfig := q.Spec.Config.LogConfig
	if logConfig == "" {
		logConfig = defaultLogConf
	}

	cm := v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      q.Name,
			Namespace: q.Namespace,
			Labels:    q.Labels,
		},
		Data: map[string]string{
			"server.conf": q.Spec.Config.ServerConfig + buildDbConfigSuffix(q, s),
			"log.conf":    logConfig,
			"mime.types":  mimetypes,
		},
	}

	if err := ctrl.SetControllerReference(q, &cm, r.Scheme); err != nil {
		panic(fmt.Sprintf("failed to set controller reference, even though we are building an object from scratch: %s", err.Error()))
	}
	return cm
}

func (r *QuestDBReconciler) reconcileConfigMap(ctx context.Context, q *crdv1beta1.QuestDB, s secrets.QuestDBSecrets) error {
	var (
		err     error
		actual  = &v1.ConfigMap{}
		desired = r.buildConfigMap(q, s)
	)

	if err = r.Get(ctx, client.ObjectKeyFromObject(q), actual); err != nil {
		if client.IgnoreNotFound(err) != nil {
			return err
		}

		if err = r.Create(ctx, &desired); err != nil {
			r.Recorder.Event(q, v1.EventTypeWarning, "ConfigMapCreateFailed", err.Error())
			return err
		}

		r.Recorder.Event(q, v1.EventTypeNormal, "ConfigMapCreated", "ConfigMap created")

		*actual = desired
	}

	// Update the ConfigMap if anything has changed
	if !reflect.DeepEqual(actual.Data, desired.Data) {
		if err = r.Update(ctx, &desired); err != nil {
			r.Recorder.Event(q, v1.EventTypeWarning, "ConfigMapUpdateFailed", err.Error())
			return err
		}

		r.Recorder.Event(q, v1.EventTypeNormal, "ConfigMapUpdated", "ConfigMap updated")
	}

	return nil
}
