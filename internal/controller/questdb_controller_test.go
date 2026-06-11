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
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	crdv1beta2 "github.com/questdb/questdb-operator/api/v1beta2"
)

var _ = Describe("QuestDB Controller", func() {
	const (
		name = "qdb-create"
		ns   = "default"
	)
	ctx := context.Background()
	var reconciler *QuestDBReconciler

	key := types.NamespacedName{Name: name, Namespace: ns}

	BeforeEach(func() {
		reconciler = &QuestDBReconciler{
			Client:   k8sClient,
			Scheme:   k8sClient.Scheme(),
			Recorder: record.NewFakeRecorder(100),
		}
	})

	AfterEach(func() {
		// envtest has no garbage collector, so delete children explicitly. Distinct names per
		// spec avoid colliding on the (immutable) PVC, which lingers under a protection finalizer.
		for _, n := range []string{"qdb-create", "qdb-restore"} {
			_ = k8sClient.Delete(ctx, &crdv1beta2.QuestDB{ObjectMeta: metav1.ObjectMeta{Name: n, Namespace: ns}})
			_ = k8sClient.Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: n + "-credentials", Namespace: ns}})
			_ = k8sClient.Delete(ctx, &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: n, Namespace: ns}})
			_ = k8sClient.Delete(ctx, &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: n, Namespace: ns}})
			_ = k8sClient.Delete(ctx, &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: n, Namespace: ns}})
		}
	})

	It("generates credentials and creates the StatefulSet, Service, and PVC", func() {
		qdb := &crdv1beta2.QuestDB{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
			Spec: crdv1beta2.QuestDBSpec{
				Image:  crdv1beta2.DefaultImage,
				Volume: crdv1beta2.QuestDBVolumeSpec{Size: resource.MustParse("1Gi")},
			},
		}
		Expect(k8sClient.Create(ctx, qdb)).To(Succeed())

		_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: key})
		Expect(err).NotTo(HaveOccurred())

		By("generating a credentials Secret")
		secret := &corev1.Secret{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name + "-credentials", Namespace: ns}, secret)).To(Succeed())
		Expect(secret.Data).To(HaveKey(envPgUser))
		Expect(secret.Data).To(HaveKey(envPgPassword))
		Expect(secret.Data[envPgPassword]).NotTo(BeEmpty())

		By("creating the StatefulSet wired to the credentials Secret")
		sts := &appsv1.StatefulSet{}
		Expect(k8sClient.Get(ctx, key, sts)).To(Succeed())
		Expect(*sts.Spec.Replicas).To(Equal(int32(1)))
		Expect(sts.Spec.Template.Spec.Containers).To(HaveLen(1))
		Expect(sts.Spec.Template.Spec.Containers[0].Name).To(Equal(containerName))
		Expect(sts.Spec.Template.Spec.Containers[0].Image).To(Equal(crdv1beta2.DefaultImage))
		Expect(sts.Spec.Template.Spec.Containers[0].EnvFrom).To(HaveLen(1))
		Expect(sts.Spec.Template.Spec.Containers[0].EnvFrom[0].SecretRef.Name).To(Equal(name + "-credentials"))
		Expect(sts.Spec.Template.Annotations).To(HaveKey(CredentialsHashAnnotation))

		By("creating the Service with the QuestDB ports")
		svc := &corev1.Service{}
		Expect(k8sClient.Get(ctx, key, svc)).To(Succeed())
		Expect(svc.Spec.Ports).To(HaveLen(4))

		By("creating the data PVC")
		pvc := &corev1.PersistentVolumeClaim{}
		Expect(k8sClient.Get(ctx, key, pvc)).To(Succeed())

		By("recording the effective credentials Secret in status")
		Expect(k8sClient.Get(ctx, key, qdb)).To(Succeed())
		Expect(qdb.Status.CredentialsSecretName).To(Equal(name + "-credentials"))
	})

	It("wires a restore initContainer when spec.volume.snapshotName is set", func() {
		rkey := types.NamespacedName{Name: "qdb-restore", Namespace: ns}
		qdb := &crdv1beta2.QuestDB{
			ObjectMeta: metav1.ObjectMeta{Name: rkey.Name, Namespace: ns},
			Spec: crdv1beta2.QuestDBSpec{
				Image:  crdv1beta2.DefaultImage,
				Volume: crdv1beta2.QuestDBVolumeSpec{Size: resource.MustParse("1Gi"), SnapshotName: "some-snapshot"},
			},
		}
		Expect(k8sClient.Create(ctx, qdb)).To(Succeed())

		_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: rkey})
		Expect(err).NotTo(HaveOccurred())

		sts := &appsv1.StatefulSet{}
		Expect(k8sClient.Get(ctx, rkey, sts)).To(Succeed())
		Expect(sts.Spec.Template.Spec.InitContainers).To(HaveLen(1))
		Expect(sts.Spec.Template.Spec.InitContainers[0].Name).To(Equal("restore-trigger"))

		pvc := &corev1.PersistentVolumeClaim{}
		Expect(k8sClient.Get(ctx, rkey, pvc)).To(Succeed())
		Expect(pvc.Spec.DataSource).NotTo(BeNil())
		Expect(pvc.Spec.DataSource.Name).To(Equal("some-snapshot"))
	})
})
