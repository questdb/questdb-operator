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
	"context"
	"fmt"
	"reflect"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	crdv1beta2 "github.com/questdb/questdb-operator/api/v1beta2"
)

// reservedConfigKeys are QuestDB server.conf settings the operator owns: they control the pg-wire
// endpoint the checkpoint/backup path connects to and the credentials it authenticates with.
// Letting a user override them via spec.config.serverConfig would silently break backups.
var reservedConfigKeys = map[string]struct{}{
	"pg.net.bind.to":       {},
	"pg.user":              {},
	"pg.password":          {},
	"http.bind.to":         {},
	"line.tcp.net.bind.to": {},
}

// nolint:unused
var questdblog = logf.Log.WithName("questdb-resource")

// SetupQuestDBWebhookWithManager registers the webhook for QuestDB in the manager.
func SetupQuestDBWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &crdv1beta2.QuestDB{}).
		WithValidator(&QuestDBCustomValidator{}).
		WithDefaulter(&QuestDBCustomDefaulter{}).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-crd-questdb-io-v1beta2-questdb,mutating=true,failurePolicy=fail,sideEffects=None,groups=crd.questdb.io,resources=questdbs,verbs=create;update,versions=v1beta2,name=mquestdb-v1beta2.kb.io,admissionReviewVersions=v1

// QuestDBCustomDefaulter sets default values on QuestDB resources.
type QuestDBCustomDefaulter struct{}

// Default applies defaults to a QuestDB.
func (d *QuestDBCustomDefaulter) Default(_ context.Context, obj *crdv1beta2.QuestDB) error {
	if obj.Spec.Image == "" {
		obj.Spec.Image = crdv1beta2.DefaultImage
	}
	if obj.Spec.ImagePullPolicy == "" {
		obj.Spec.ImagePullPolicy = corev1.PullIfNotPresent
	}
	if obj.Spec.PodSecurityContext == nil {
		obj.Spec.PodSecurityContext = &corev1.PodSecurityContext{}
	}
	if obj.Spec.PodSecurityContext.FSGroup == nil {
		obj.Spec.PodSecurityContext.FSGroup = ptr.To(crdv1beta2.DefaultFSGroup)
	}
	return nil
}

// +kubebuilder:webhook:path=/validate-crd-questdb-io-v1beta2-questdb,mutating=false,failurePolicy=fail,sideEffects=None,groups=crd.questdb.io,resources=questdbs,verbs=create;update,versions=v1beta2,name=vquestdb-v1beta2.kb.io,admissionReviewVersions=v1

// QuestDBCustomValidator validates QuestDB resources.
type QuestDBCustomValidator struct{}

// ValidateCreate validates a QuestDB on creation.
func (v *QuestDBCustomValidator) ValidateCreate(_ context.Context, obj *crdv1beta2.QuestDB) (admission.Warnings, error) {
	return nil, validateQuestDBSpec(obj)
}

// ValidateUpdate validates a QuestDB on update, enforcing immutability of volume identity fields.
func (v *QuestDBCustomValidator) ValidateUpdate(_ context.Context, oldObj, newObj *crdv1beta2.QuestDB) (admission.Warnings, error) {
	oldVol := oldObj.Spec.Volume
	newVol := newObj.Spec.Volume

	if newVol.Size.Cmp(oldVol.Size) < 0 {
		return nil, fmt.Errorf("spec.volume.size cannot be shrunk")
	}
	if newVol.VolumeName != oldVol.VolumeName {
		return nil, fmt.Errorf("spec.volume.volumeName is immutable")
	}
	if newVol.SnapshotName != oldVol.SnapshotName {
		return nil, fmt.Errorf("spec.volume.snapshotName is immutable")
	}
	if !reflect.DeepEqual(newVol.StorageClassName, oldVol.StorageClassName) {
		return nil, fmt.Errorf("spec.volume.storageClassName is immutable")
	}
	if !reflect.DeepEqual(newVol.Selector, oldVol.Selector) {
		return nil, fmt.Errorf("spec.volume.selector is immutable")
	}
	return nil, validateQuestDBSpec(newObj)
}

// validateQuestDBSpec holds the create/update-invariant checks for a QuestDB spec.
func validateQuestDBSpec(obj *crdv1beta2.QuestDB) error {
	if obj.Spec.Volume.Size.Sign() <= 0 {
		return fmt.Errorf("spec.volume.size must be greater than 0")
	}
	// A selector statically binds to an existing PV; a snapshotName provisions a new volume from a
	// snapshot. Both on one PVC is contradictory and leaves the pod unschedulable.
	if obj.Spec.Volume.Selector != nil && obj.Spec.Volume.SnapshotName != "" {
		return fmt.Errorf("spec.volume.selector and spec.volume.snapshotName are mutually exclusive")
	}
	for _, e := range obj.Spec.ExtraEnv {
		if e.Name == crdv1beta2.EnvPgUser || e.Name == crdv1beta2.EnvPgPassword {
			return fmt.Errorf("spec.extraEnv may not set %q; pg-wire credentials are managed by the operator via spec.auth", e.Name)
		}
	}
	if key, ok := overridesReservedConfig(obj.Spec.Config.ServerConfig); ok {
		return fmt.Errorf("spec.config.serverConfig may not set operator-managed key %q", key)
	}
	return nil
}

// overridesReservedConfig reports the first reserved key set by a server.conf body, if any.
func overridesReservedConfig(serverConfig string) (string, bool) {
	for line := range strings.SplitSeq(serverConfig, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		key, _, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		if _, reserved := reservedConfigKeys[strings.TrimSpace(key)]; reserved {
			return strings.TrimSpace(key), true
		}
	}
	return "", false
}

// ValidateDelete is a no-op for QuestDB.
func (v *QuestDBCustomValidator) ValidateDelete(_ context.Context, _ *crdv1beta2.QuestDB) (admission.Warnings, error) {
	return nil, nil
}
