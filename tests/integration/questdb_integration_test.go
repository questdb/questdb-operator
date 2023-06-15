package integration

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"

	crdv1beta1 "github.com/questdb/questdb-operator/api/v1beta1"
	testutils "github.com/questdb/questdb-operator/tests/utils"
)

var _ = Describe("QuestDB Integration Test", func() {
	var (
		interval = time.Second
	)
	Context("when creating a QuestDB", Ordered, func() {
		var (
			q       *crdv1beta1.QuestDB
			timeout = time.Minute
		)
		BeforeAll(func() {
			q = testutils.BuildMockQuestDB(ctx, k8sClient)
			q.Spec.Volume.StorageClassName = pointer.String("standard")
			By("Creating a QuestDB")
			Expect(k8sClient.Create(ctx, q)).To(Succeed())
		})

		It("should create a service", func() {
			svc := &v1.Service{}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(q), svc)).To(Succeed())
			}, timeout, interval).Should(Succeed())

			Expect(svc.Spec.Ports).To(HaveLen(4))
			Expect(svc.OwnerReferences).To(HaveLen(1))
			Expect(svc.OwnerReferences[0].Kind).To(Equal("QuestDB"))
			Expect(svc.OwnerReferences[0].Name).To(Equal(q.Name))
		})

		It("should create a PVC", func() {
			pvc := &v1.PersistentVolumeClaim{}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(q), pvc)).To(Succeed())
			}, timeout, interval).Should(Succeed())

			Expect(pvc.OwnerReferences).To(HaveLen(1))
			Expect(pvc.OwnerReferences[0].Kind).To(Equal("QuestDB"))
			Expect(pvc.OwnerReferences[0].Name).To(Equal(q.Name))
		})

		It("should create a configmap", func() {
			cm := &v1.ConfigMap{}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(q), cm)).To(Succeed())
			}, timeout, interval).Should(Succeed())

			Expect(cm.OwnerReferences).To(HaveLen(1))
			Expect(cm.OwnerReferences[0].Kind).To(Equal("QuestDB"))
			Expect(cm.OwnerReferences[0].Name).To(Equal(q.Name))
		})

		It("should create a statefulset that is eventually ready", func() {
			sts := &appsv1.StatefulSet{}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(q), sts)).To(Succeed())
			}, timeout, interval).Should(Succeed())

			Expect(sts.Spec.Template.Spec.Containers).To(HaveLen(1))
			Expect(sts.OwnerReferences).To(HaveLen(1))
			Expect(sts.OwnerReferences[0].Kind).To(Equal("QuestDB"))
			Expect(sts.OwnerReferences[0].Name).To(Equal(q.Name))

			By("The statefulset should be ready")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(q), sts)).To(Succeed())
				g.Expect(sts.Status.ReadyReplicas).To(Equal(int32(1)))
			}, timeout, interval).Should(Succeed())
		})

	})
})
