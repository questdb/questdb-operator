package controller

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	crdv1beta1 "github.com/questdb/questdb-operator/api/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	testutils "github.com/questdb/questdb-operator/tests/utils"
)

var _ = Describe("QuestDB Controller", func() {
	var (
		timeout            = time.Second * 2
		consistencyTimeout = time.Millisecond * 500
		interval           = time.Millisecond * 100
	)

	It("should update the pvc size when the spec changes", func() {
		Skip("this test doesn't work in envtest, but it is tested against the ebs csi")

		By("Creating a storageclass that allows resizing")
		testutils.BuildAndCreateMockStorageClass(ctx, k8sClient)

		By("Creating a new QuestDB")
		q := testutils.BuildAndCreateMockQuestDB(ctx, k8sClient)

		By("Verifying the pvc has been created")
		pvc := &v1.PersistentVolumeClaim{}
		Eventually(func() error {
			return k8sClient.Get(ctx, client.ObjectKey{Name: q.Name, Namespace: q.Namespace}, pvc)
		}, timeout, interval).Should(Succeed())

		Expect(pvc.Spec.Resources.Requests.Storage().String()).To(Equal("10Gi"))

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

	It("should prevent PVC size from shrinking", func() {
		By("Creating a new QuestDB")
		q := testutils.BuildAndCreateMockQuestDB(ctx, k8sClient)

		By("Verifying the pvc has been created")
		pvc := &v1.PersistentVolumeClaim{}
		Eventually(func() error {
			return k8sClient.Get(ctx, client.ObjectKey{Name: q.Name, Namespace: q.Namespace}, pvc)
		}, timeout, interval).Should(Succeed())

		Expect(pvc.Spec.Resources.Requests.Storage().String()).To(Equal("10Gi"))

		By("Updating the QuestDB spec")
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: q.Name, Namespace: q.Namespace}, q)).To(Succeed())
			q.Spec.Volume.Size = resource.MustParse("2Gi")
			g.Expect(k8sClient.Update(ctx, q)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		By("Verifying the pvc has not been resized")
		Consistently(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: q.Name, Namespace: q.Namespace}, pvc)).To(Succeed())
			g.Expect(pvc.Spec.Resources.Requests.Storage().String()).To(Equal("10Gi"))
		}, consistencyTimeout, interval).Should(Succeed())

	})

	Context("port allocation", func() {
		var (
			q *crdv1beta1.QuestDB
		)
		BeforeEach(func() {
			By("Creating a new QuestDB")
			q = testutils.BuildAndCreateMockQuestDB(ctx, k8sClient)
		})

		It("Should have the correct default ports", func() {

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

	Context("statefulset updated", func() {

		var (
			q   *crdv1beta1.QuestDB
			sts *appsv1.StatefulSet
		)

		BeforeEach(func() {
			By("Creating a new QuestDB")
			q = testutils.BuildAndCreateMockQuestDB(ctx, k8sClient)

			By("Verifying the statefulset has been created")
			sts = &appsv1.StatefulSet{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: q.Name, Namespace: q.Namespace}, sts)
			}, timeout, interval).Should(Succeed())
		})

		It("should prevent annotation changes directly on the statefulset", func() {
			By("Verifying the statefulset has been created")
			sts := &appsv1.StatefulSet{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: q.Name, Namespace: q.Namespace}, sts)
			}, timeout, interval).Should(Succeed())

			By("Verifying the statefulset has no annotations")
			Expect(sts.Annotations).To(BeEmpty())

			By("Adding an annotation directly to the statefulset")
			Eventually(func(g Gomega) {
				sts.Annotations = map[string]string{"foo": "bar"}
				g.Expect(k8sClient.Update(ctx, sts)).To(Succeed())
			}, timeout, interval).Should(Succeed())

			By("Verifying the annotation has been removed")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: q.Name, Namespace: q.Namespace}, sts)).To(Succeed())
				g.Expect(sts.Annotations).ToNot(HaveKeyWithValue("foo", "bar"))
			}, timeout, interval).Should(Succeed())

		})

		It("should update the statefulset if annotations change", func() {

			By("Verifying the statefulset has no annotations")
			Expect(sts.Annotations).To(BeEmpty())

			By("Adding an annotation to the QuestDB")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: q.Name, Namespace: q.Namespace}, q)).To(Succeed())
				q.Spec.StatefulSetAnnotations = map[string]string{"foo": "bar"}
				g.Expect(k8sClient.Update(ctx, q)).To(Succeed())
			}, timeout, interval).Should(Succeed())

			By("Verifying the annotation has been added")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: q.Name, Namespace: q.Namespace}, sts)).To(Succeed())
				g.Expect(sts.Annotations).To(HaveKeyWithValue("foo", "bar"))
			}, timeout, interval).Should(Succeed())

		})

		It("should successfully add an extra volume and mount to the statefulset", func() {
			By("Verifying the statefulset has only 1 volume and mount")
			Expect(sts.Spec.Template.Spec.Volumes).To(HaveLen(2))
			Expect(sts.Spec.Template.Spec.Containers[0].VolumeMounts).To(HaveLen(2))

			By("Adding an extra volume to the QuestDB")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: q.Name, Namespace: q.Namespace}, q)).To(Succeed())
				q.Spec.ExtraVolumeMounts = append(q.Spec.ExtraVolumeMounts, v1.VolumeMount{
					Name:      "extra-volume",
					MountPath: "/extra",
				})
				q.Spec.ExtraVolumes = append(q.Spec.ExtraVolumes, v1.Volume{
					Name: "extra-volume",
					VolumeSource: v1.VolumeSource{
						EmptyDir: &v1.EmptyDirVolumeSource{},
					},
				})
				g.Expect(k8sClient.Update(ctx, q)).To(Succeed())
			}, timeout, interval).Should(Succeed())

			By("Verifying the extra volume and mount have been added")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: q.Name, Namespace: q.Namespace}, sts)).To(Succeed())
				Expect(sts.Spec.Template.Spec.Volumes).To(HaveLen(3))
				Expect(sts.Spec.Template.Spec.Containers[0].VolumeMounts).To(HaveLen(3))
			}, timeout, interval).Should(Succeed())
		})

		It("should update the statefulset on image change", func() {
			By("Changing the image")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: q.Name, Namespace: q.Namespace}, q)).To(Succeed())
				q.Spec.Image = "questdb/questdb:a.b.c"
				g.Expect(k8sClient.Update(ctx, q)).To(Succeed())
			}, timeout, interval).Should(Succeed())

			By("Verifying the statefulset has been updated")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: q.Name, Namespace: q.Namespace}, sts)).To(Succeed())
				g.Expect(sts.Spec.Template.Spec.Containers[0].Image).To(Equal("questdb/questdb:a.b.c"))
			}, timeout, interval).Should(Succeed())
		})

	})

	Context("pgauth updates", Ordered, func() {
		var (
			secretName = "test-secret"
			q          = &crdv1beta1.QuestDB{}
		)

		BeforeAll(func() {
			By("Creating a new QuestDB")
			q = testutils.BuildAndCreateMockQuestDB(ctx, k8sClient)
		})

		It("should update the statefulset when new pg auth secret is created", func() {

			By("Verifying the statefulset has been created")
			sts := &appsv1.StatefulSet{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: q.Name, Namespace: q.Namespace}, sts)
			}, timeout, interval).Should(Succeed())

			By("Adding the secret")
			Eventually(func(g Gomega) {
				s := &v1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      secretName,
						Namespace: q.Namespace,
						Annotations: map[string]string{
							crdv1beta1.AnnotationQuestDBName:       q.Name,
							crdv1beta1.AnnotationQuestDBSecretType: "psql",
						},
					},
					StringData: map[string]string{
						"QDB_PG_USER":     "test-username",
						"QDB_PG_PASSWORD": "test-password",
					},
				}
				Expect(k8sClient.Create(ctx, s)).Should(Succeed())

			}, timeout, interval).Should(Succeed())

			By("Verifying the statefulset has been updated")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: q.Name, Namespace: q.Namespace}, sts)).To(Succeed())
				g.Expect(sts.Spec.Template.Spec.Containers[0].EnvFrom).Should(HaveLen(1))
				g.Expect(sts.Spec.Template.Spec.Containers[0].EnvFrom[0].SecretRef).ShouldNot(BeNil())
				g.Expect(sts.Spec.Template.Spec.Containers[0].EnvFrom[0].SecretRef.Name).Should(Equal(secretName))
			}, timeout, interval).Should(Succeed())
		})
	})

	It("should update the readyreplicas status on the questdb", func() {
		By("Creating a new QuestDB")
		q := testutils.BuildAndCreateMockQuestDB(ctx, k8sClient)

		By("Verifying the readyreplicas status is 0")
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: q.Name, Namespace: q.Namespace}, q)).To(Succeed())
			g.Expect(q.Status.StatefulSetReadyReplicas).To(Equal(0))
		}, timeout, interval).Should(Succeed())

		By("Getting the statefulset")
		sts := &appsv1.StatefulSet{}
		Eventually(func() error {
			return k8sClient.Get(ctx, client.ObjectKey{Name: q.Name, Namespace: q.Namespace}, sts)
		}, timeout, interval).Should(Succeed())

		By("Updating the statefulset status's readyreplicas to 1")
		Eventually(func(g Gomega) {
			sts.Status.Replicas = 1
			sts.Status.ReadyReplicas = 1
			g.Expect(k8sClient.Status().Update(ctx, sts)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		By("Verifying the readyreplicas status is 1")
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: q.Name, Namespace: q.Namespace}, q)).To(Succeed())
			g.Expect(q.Status.StatefulSetReadyReplicas).To(Equal(1))
		}, timeout, interval).Should(Succeed())

	})

})
