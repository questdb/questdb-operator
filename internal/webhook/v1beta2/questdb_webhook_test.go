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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	crdv1beta2 "github.com/questdb/questdb-operator/api/v1beta2"
)

var _ = Describe("QuestDB Webhook", func() {
	const ns = "default"

	get := func(name string) *crdv1beta2.QuestDB {
		q := &crdv1beta2.QuestDB{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: ns}, q)).To(Succeed())
		return q
	}

	AfterEach(func() {
		_ = k8sClient.DeleteAllOf(ctx, &crdv1beta2.QuestDB{}, client.InNamespace(ns))
	})

	It("defaults image, pull policy, and fsGroup", func() {
		qdb := &crdv1beta2.QuestDB{
			ObjectMeta: metav1.ObjectMeta{Name: "wh-default", Namespace: ns},
			Spec:       crdv1beta2.QuestDBSpec{Volume: crdv1beta2.QuestDBVolumeSpec{Size: resource.MustParse("1Gi")}},
		}
		Expect(k8sClient.Create(ctx, qdb)).To(Succeed())

		f := get("wh-default")
		Expect(f.Spec.Image).To(Equal(crdv1beta2.DefaultImage))
		Expect(f.Spec.ImagePullPolicy).To(Equal(corev1.PullIfNotPresent))
		Expect(f.Spec.PodSecurityContext).NotTo(BeNil())
		Expect(f.Spec.PodSecurityContext.FSGroup).NotTo(BeNil())
		Expect(*f.Spec.PodSecurityContext.FSGroup).To(Equal(crdv1beta2.DefaultFSGroup))
	})

	It("rejects a volume size of zero", func() {
		qdb := &crdv1beta2.QuestDB{
			ObjectMeta: metav1.ObjectMeta{Name: "wh-zero", Namespace: ns},
			Spec:       crdv1beta2.QuestDBSpec{Volume: crdv1beta2.QuestDBVolumeSpec{Size: resource.MustParse("0")}},
		}
		Expect(k8sClient.Create(ctx, qdb)).NotTo(Succeed())
	})

	It("enforces no-shrink and volume immutability on update", func() {
		qdb := &crdv1beta2.QuestDB{
			ObjectMeta: metav1.ObjectMeta{Name: "wh-immutable", Namespace: ns},
			Spec: crdv1beta2.QuestDBSpec{Volume: crdv1beta2.QuestDBVolumeSpec{
				Size:             resource.MustParse("2Gi"),
				StorageClassName: ptr.To("fast"),
			}},
		}
		Expect(k8sClient.Create(ctx, qdb)).To(Succeed())

		By("rejecting a shrink")
		f := get("wh-immutable")
		f.Spec.Volume.Size = resource.MustParse("1Gi")
		Expect(k8sClient.Update(ctx, f)).NotTo(Succeed())

		By("allowing a grow")
		f = get("wh-immutable")
		f.Spec.Volume.Size = resource.MustParse("3Gi")
		Expect(k8sClient.Update(ctx, f)).To(Succeed())

		By("rejecting a storage class change")
		f = get("wh-immutable")
		f.Spec.Volume.StorageClassName = ptr.To("slow")
		Expect(k8sClient.Update(ctx, f)).NotTo(Succeed())
	})

	It("rejects an extraEnv that overrides operator-managed pg credentials", func() {
		qdb := &crdv1beta2.QuestDB{
			ObjectMeta: metav1.ObjectMeta{Name: "wh-env", Namespace: ns},
			Spec: crdv1beta2.QuestDBSpec{
				Volume:   crdv1beta2.QuestDBVolumeSpec{Size: resource.MustParse("1Gi")},
				ExtraEnv: []corev1.EnvVar{{Name: crdv1beta2.EnvPgPassword, Value: "hunter2"}},
			},
		}
		Expect(k8sClient.Create(ctx, qdb)).NotTo(Succeed())
	})

	It("rejects a serverConfig that overrides an operator-managed key", func() {
		qdb := &crdv1beta2.QuestDB{
			ObjectMeta: metav1.ObjectMeta{Name: "wh-conf", Namespace: ns},
			Spec: crdv1beta2.QuestDBSpec{
				Volume: crdv1beta2.QuestDBVolumeSpec{Size: resource.MustParse("1Gi")},
				Config: crdv1beta2.QuestDBConfigSpec{ServerConfig: "shared.worker.count=2\npg.net.bind.to=0.0.0.0:9999\n"},
			},
		}
		Expect(k8sClient.Create(ctx, qdb)).NotTo(Succeed())
	})

	It("rejects volume.selector together with volume.snapshotName", func() {
		qdb := &crdv1beta2.QuestDB{
			ObjectMeta: metav1.ObjectMeta{Name: "wh-both", Namespace: ns},
			Spec: crdv1beta2.QuestDBSpec{
				Volume: crdv1beta2.QuestDBVolumeSpec{
					Size:         resource.MustParse("1Gi"),
					Selector:     &metav1.LabelSelector{MatchLabels: map[string]string{"disk": "ssd"}},
					SnapshotName: "snap-1",
				},
			},
		}
		Expect(k8sClient.Create(ctx, qdb)).NotTo(Succeed())
	})
})
