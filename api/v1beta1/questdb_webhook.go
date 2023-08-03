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
	"bufio"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var questdblog = logf.Log.WithName("questdb-resource")

func (r *QuestDB) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//+kubebuilder:webhook:path=/mutate-crd-questdb-io-v1beta1-questdb,mutating=true,failurePolicy=fail,sideEffects=None,groups=crd.questdb.io,resources=questdbs,verbs=create;update,versions=v1beta1,name=mquestdb.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &QuestDB{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *QuestDB) Default() {
	questdblog.Info("default", "name", r.Name)

	// TODO(user): fill in your defaulting logic.
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-crd-questdb-io-v1beta1-questdb,mutating=false,failurePolicy=fail,sideEffects=None,groups=crd.questdb.io,resources=questdbs,verbs=create;update,versions=v1beta1,name=vquestdb.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &QuestDB{}

var reservedConfigKeys = []string{
	"http.bind.to",
	"line.tcp.net.bind.to",
	"pg.net.bind.to",
	"pg.user",
	"pg.password",
	"line.tcp.auth.db.path",
}

func validateDbConfig(config string) error {
	scanner := bufio.NewScanner(strings.NewReader(config))
	for scanner.Scan() {
		line := scanner.Text()
		for _, reservedKey := range reservedConfigKeys {
			if strings.HasPrefix(line, reservedKey) {
				return fmt.Errorf("config key %s is reserved", reservedKey)
			}
		}
	}
	return nil
}

func validateVolumeSpec(config QuestDBVolumeSpec) error {
	if config.Size.Value() == 0 {
		return fmt.Errorf("volume size must be greater than 0")
	}

	if config.SnapshotName != "" && config.Selector != nil {
		return fmt.Errorf("cannot specify both snapshot name and selector")
	}

	return nil

}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *QuestDB) ValidateCreate() error {
	questdblog.Info("validate create", "name", r.Name)

	if r.Spec.Volume.Size.Value() == 0 {
		return errors.New("volume size must be greater than 0")
	}

	if r.Spec.Config.ServerConfig != "" {
		if err := validateDbConfig(r.Spec.Config.ServerConfig); err != nil {
			return err
		}
	}

	if err := validateVolumeSpec(r.Spec.Volume); err != nil {
		return err
	}

	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *QuestDB) ValidateUpdate(old runtime.Object) error {
	questdblog.Info("validate update", "name", r.Name)

	oldQdb, valid := old.(*QuestDB)
	if !valid {
		return errors.New("old object is not a QuestDB")
	}

	if r.Spec.Volume.Size.Value() < oldQdb.Spec.Volume.Size.Value() {
		return errors.New("cannot shrink volume size")
	}

	if r.Spec.Config.ServerConfig != "" {
		if err := validateDbConfig(r.Spec.Config.ServerConfig); err != nil {
			return err
		}
	}

	// Check that volume specs haven't changed (with the exception of size)
	if r.Spec.Volume.SnapshotName != oldQdb.Spec.Volume.SnapshotName {
		return errors.New("cannot change snapshot name")
	}

	// Check that the volume selector hasn't changed
	if !reflect.DeepEqual(r.Spec.Volume.Selector, oldQdb.Spec.Volume.Selector) {
		return errors.New("cannot change snapshot selector")
	}

	// Also check if there was a volume selector in the first place
	if reflect.ValueOf(r.Spec.Volume.Selector) != reflect.ValueOf(oldQdb.Spec.Volume.Selector) {
		return errors.New("cannot change snapshot selector")
	}

	if r.Spec.Volume.VolumeName != oldQdb.Spec.Volume.VolumeName {
		return errors.New("cannot change volume name")
	}

	if reflect.ValueOf(r.Spec.Volume.StorageClassName) != reflect.ValueOf(oldQdb.Spec.Volume.StorageClassName) {
		return errors.New("cannot change storage class name")
	}

	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *QuestDB) ValidateDelete() error {
	questdblog.Info("validate delete", "name", r.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil
}
