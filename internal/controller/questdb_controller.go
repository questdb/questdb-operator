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
	cm, err := r.buildConfigMap(q, secrets)
	if err != nil {
		return ctrl.Result{}, err
	}
	if err = r.Get(ctx, req.NamespacedName, &cm); err != nil {
		if client.IgnoreNotFound(err) != nil {
			return ctrl.Result{}, err
		}

		if err = r.Create(ctx, &cm); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Update the ConfigMap if needed
	var configChanged, portsChanged bool

	// todo: this reconciliation method isn't great.. fix this
	// Use strings.HasPrefix to check non-reserved values since we add some reserved values at the end
	if !strings.HasPrefix(cm.Data["server.conf"], q.Spec.Config.ServerConfig) {
		cm.Data["server.conf"] = q.Spec.Config.ServerConfig
		configChanged = true
	}
	// Use strings.HasSuffix to check the reserved values
	if !strings.HasSuffix(cm.Data["server.conf"], buildDbConfigSuffix(q, secrets)) {
		cm.Data["server.conf"] = q.Spec.Config.ServerConfig + buildDbConfigSuffix(q, secrets)
		configChanged = true
		portsChanged = true
	}

	// Check reserved values
	// todo: refactor this too
	if cm.Data["log.conf"] != q.Spec.Config.LogConfig {
		if q.Spec.Config.LogConfig == "" {
			q.Spec.Config.LogConfig = defaultLogConf
		} else {
			cm.Data["log.conf"] = q.Spec.Config.LogConfig
		}
		configChanged = true
	}

	if configChanged {
		if err = r.Update(ctx, &cm); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Reconcile the PVC
	pvc, err := r.buildPvc(q)
	if err != nil {
		return ctrl.Result{}, err
	}

	if err = r.Get(ctx, req.NamespacedName, &pvc); err != nil {
		if client.IgnoreNotFound(err) != nil {
			return ctrl.Result{}, err
		}

		if err = r.Create(ctx, &pvc); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Resize the PVC if needed
	if pvc.Spec.Resources.Requests[v1.ResourceStorage] != q.Spec.Volume.Size {
		if err = r.Update(ctx, &pvc); err != nil {
			return ctrl.Result{}, err
		}

		pvc.Spec.Resources.Requests = v1.ResourceList{
			v1.ResourceStorage: q.Spec.Volume.Size,
		}
		if err = r.Update(ctx, &pvc); err != nil {
			r.Recorder.Event(q, v1.EventTypeWarning, "PVCResizeFailed", err.Error())
			return ctrl.Result{}, err
		}
	}

	// Reconcile the StatefulSet
	sts, err := r.buildStatefulSet(q, secrets)
	if err != nil {
		return ctrl.Result{}, err
	}
	if err = r.Get(ctx, req.NamespacedName, &sts); err != nil {
		if client.IgnoreNotFound(err) != nil {
			return ctrl.Result{}, err
		}

		if err = r.Create(ctx, &sts); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Update the StatefulSet image if needed
	if sts.Spec.Template.Spec.Containers[0].Image != q.Spec.Image {
		sts.Spec.Template.Spec.Containers[0].Image = q.Spec.Image
		if err = r.Update(ctx, &sts); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Reconcile the Service
	svc, err := r.buildService(q)
	if err != nil {
		return ctrl.Result{}, err
	}
	if err = r.Get(ctx, req.NamespacedName, &svc); err != nil {
		if client.IgnoreNotFound(err) != nil {
			return ctrl.Result{}, err
		}

		if err = r.Create(ctx, &svc); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Update the Service ports if needed
	if portsChanged {
		svc, err = r.buildService(q)
		if err != nil {
			return ctrl.Result{}, err
		}
		if err = r.Update(ctx, &svc); err != nil {
			return ctrl.Result{}, err
		}
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

func (r *QuestDBReconciler) buildStatefulSet(q *crdv1beta1.QuestDB, s secrets.QuestDBSecrets) (appsv1.StatefulSet, error) {
	var err error

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
									ContainerPort: q.PortHttp(),
								},
								{
									Name:          "psql",
									ContainerPort: q.PortPsql(),
								},
								{
									Name:          "ilp",
									ContainerPort: q.PortIlp(),
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
			SubPath:   "auth.json",
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

	err = ctrl.SetControllerReference(q, &sts, r.Scheme)
	return sts, err

}

func (r *QuestDBReconciler) buildService(q *crdv1beta1.QuestDB) (v1.Service, error) {
	var err error

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
					Port: q.PortHttp(),
				},
				{
					Name: "psql",
					Port: q.PortPsql(),
				},
				{
					Name: "ilp",
					Port: q.PortIlp(),
				},
				{
					Name: "metrics",
					Port: 9003,
				},
			},
			Selector: q.ObjectMeta.Labels,
		},
	}

	err = ctrl.SetControllerReference(q, &svc, r.Scheme)
	return svc, err
}

func (r *QuestDBReconciler) buildPvc(q *crdv1beta1.QuestDB) (v1.PersistentVolumeClaim, error) {
	var err error

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

	err = ctrl.SetControllerReference(q, &pvc, r.Scheme)

	return pvc, err

}

func buildDbConfigSuffix(q *crdv1beta1.QuestDB, secrets secrets.QuestDBSecrets) string {
	dbConfig := strings.Builder{}
	dbConfig.WriteRune('\n')
	dbConfig.WriteString("### Reserved values -- set by the operator ###\n")
	dbConfig.WriteString(fmt.Sprintf("http.bind.to=0.0.0.0:%d\n", q.PortHttp()))
	dbConfig.WriteString(fmt.Sprintf("line.tcp.net.bind.to=0.0.0.0:%d\n", q.PortIlp()))
	dbConfig.WriteString(fmt.Sprintf("pg.net.bind.to=0.0.0.0:%d\n", q.PortPsql()))

	if secrets.IlpSecret != nil {
		dbConfig.WriteString(fmt.Sprintf("ilp.auth.file=%s\n", "/var/lib/questdb/auth/auth.json")) // todo: make auth.json location/name configurable?
	}

	return dbConfig.String()
}

func (r *QuestDBReconciler) buildConfigMap(q *crdv1beta1.QuestDB, s secrets.QuestDBSecrets) (v1.ConfigMap, error) {
	// todo: Run some validation on the config, probably in the webhook
	// todo: Probably move credentials to a secret
	var err error

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
			// todo: add default log conf if nothing is specified
			"log.conf":   logConfig,
			"mime.types": mimetypes,
		},
	}

	err = ctrl.SetControllerReference(q, &cm, r.Scheme)
	return cm, err
}
