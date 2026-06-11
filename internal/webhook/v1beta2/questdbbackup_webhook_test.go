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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	crdv1beta2 "github.com/questdb/questdb-operator/api/v1beta2"
)

var _ = Describe("QuestDBBackup Webhook", func() {
	const ns = "default"

	AfterEach(func() {
		_ = k8sClient.DeleteAllOf(ctx, &crdv1beta2.QuestDBBackup{}, client.InNamespace(ns))
	})

	It("defaults method to VolumeSnapshot", func() {
		b := &crdv1beta2.QuestDBBackup{
			ObjectMeta: metav1.ObjectMeta{Name: "whb-default", Namespace: ns},
			Spec:       crdv1beta2.QuestDBBackupSpec{QuestDBName: "q"},
		}
		Expect(k8sClient.Create(ctx, b)).To(Succeed())

		f := &crdv1beta2.QuestDBBackup{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "whb-default", Namespace: ns}, f)).To(Succeed())
		Expect(f.Spec.Method).To(Equal(crdv1beta2.BackupMethodVolumeSnapshot))
	})

	It("rejects an empty questdbName", func() {
		b := &crdv1beta2.QuestDBBackup{
			ObjectMeta: metav1.ObjectMeta{Name: "whb-empty", Namespace: ns},
			Spec:       crdv1beta2.QuestDBBackupSpec{QuestDBName: "", Method: crdv1beta2.BackupMethodVolumeSnapshot},
		}
		Expect(k8sClient.Create(ctx, b)).NotTo(Succeed())
	})

	It("enforces immutability of questdbName", func() {
		b := &crdv1beta2.QuestDBBackup{
			ObjectMeta: metav1.ObjectMeta{Name: "whb-immutable", Namespace: ns},
			Spec:       crdv1beta2.QuestDBBackupSpec{QuestDBName: "q", Method: crdv1beta2.BackupMethodVolumeSnapshot},
		}
		Expect(k8sClient.Create(ctx, b)).To(Succeed())

		f := &crdv1beta2.QuestDBBackup{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "whb-immutable", Namespace: ns}, f)).To(Succeed())
		f.Spec.QuestDBName = "other"
		Expect(k8sClient.Update(ctx, f)).NotTo(Succeed())
	})
})
