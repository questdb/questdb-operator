package controller

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	crdv1beta1 "github.com/questdb/questdb-operator/api/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("QuestDB Controller", func() {
	var (
		timeout            = time.Second * 2
		consistencyTimeout = time.Millisecond * 600
		interval           = time.Millisecond * 100
	)

	It("should update the pvc size when the spec changes", func() {
		Skip("this test doesn't work in envtest, but it is tested against the ebs csi")

		By("Creating a storageclass that allows resizing")
		buildMockStorageClass()

		By("Creating a new QuestDB")
		q := buildMockQuestDB()

		By("Verifying the pvc has been created")
		pvc := &v1.PersistentVolumeClaim{}
		Eventually(func() error {
			return k8sClient.Get(ctx, client.ObjectKey{Name: q.Name, Namespace: q.Namespace}, pvc)
		}, timeout, interval).Should(Succeed())

		Expect(pvc.Spec.Resources.Requests.Storage().String()).To(Equal("1Gi"))

		By("Updating the QuestDB spec")
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: q.Name, Namespace: q.Namespace}, q)).To(Succeed())
			q.Spec.Volume.Size = resource.MustParse("2Gi")
			g.Expect(k8sClient.Update(ctx, q)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		By("Verifying the pvc has been resized")
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: q.Name, Namespace: q.Namespace}, pvc)).To(Succeed())
			g.Expect(pvc.Spec.Resources.Requests.Storage().String()).To(Equal("2Gi"))
		}, timeout, interval).Should(Succeed())

	})

	Context("port allocation", func() {
		var (
			q *crdv1beta1.QuestDB
		)
		BeforeEach(func() {
			By("Creating a new QuestDB")
			q = buildMockQuestDB()
		})

		It("Should have the correct default ports", func() {
			By("check the spec port values -- they are 0 by default")
			Expect(q.Spec.Ports.Ilp).To(Equal(int32(0)))
			Expect(q.Spec.Ports.Psql).To(Equal(int32(0)))
			Expect(q.Spec.Ports.Http).To(Equal(int32(0)))

			By("check the configmap port values")
			cm := &v1.ConfigMap{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: q.Name, Namespace: q.Namespace}, cm)
			}, timeout, interval).Should(Succeed())

			By("Check the service port values")
			svc := &v1.Service{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: q.Name, Namespace: q.Namespace}, svc)
			}, timeout, interval).Should(Succeed())

			Expect(svc.Spec.Ports).To(ContainElements(
				v1.ServicePort{
					Name:       "ilp",
					Port:       9009,
					TargetPort: intstr.FromInt(9009),
					Protocol:   v1.ProtocolTCP,
				},
				v1.ServicePort{
					Name:       "psql",
					Port:       8812,
					TargetPort: intstr.FromInt(8812),
					Protocol:   v1.ProtocolTCP,
				},
				v1.ServicePort{
					Name:       "http",
					Port:       9000,
					TargetPort: intstr.FromInt(9000),
					Protocol:   v1.ProtocolTCP,
				},
			))
		})

	})

	It("should not reconcile annotations on the statefulset", func() {
		By("Creating a new QuestDB")
		q := buildMockQuestDB()

		By("Verifying the statefulset has been created")
		sts := &appsv1.StatefulSet{}
		Eventually(func() error {
			return k8sClient.Get(ctx, client.ObjectKey{Name: q.Name, Namespace: q.Namespace}, sts)
		}, timeout, interval).Should(Succeed())

		By("Verifying the statefulset has no annotations")
		Expect(sts.Annotations).To(BeEmpty())

		By("Adding an annotation to the statefulset")
		Eventually(func(g Gomega) {
			sts.Annotations = map[string]string{"foo": "bar"}
			g.Expect(k8sClient.Update(ctx, sts)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		By("Verifying the statefulset still has the annotation")
		Consistently(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: q.Name, Namespace: q.Namespace}, sts)).To(Succeed())
			g.Expect(sts.Annotations).To(HaveKeyWithValue("foo", "bar"))
		}, consistencyTimeout, interval).Should(Succeed())

	})

})
