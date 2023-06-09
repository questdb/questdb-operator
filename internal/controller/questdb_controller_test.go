package controller

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("QuestDB Controller", func() {
	var (
		timeout = time.Second * 2
		//consistencyTimeout = time.Millisecond * 600
		interval = time.Millisecond * 100
	)

	It("should update the pvc size when the spec changes", func() {
		Skip("this test doesn't work in envtest, but it is tested against the ebs csi")
		By("Creating a storageclass that allows resizing")
		buildMockStorageClass()

		By("Creating a new QuestDB")
		ctx := context.Background()
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

})
