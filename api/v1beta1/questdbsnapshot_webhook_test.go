package v1beta1

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/pointer"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("QuestDBSnapshot Webhook", func() {
	var (
		q    *QuestDB
		snap *QuestDBSnapshot
	)
	BeforeEach(func() {
		q = &QuestDB{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("test-%d", time.Now().UnixNano()),
				Namespace: "default",
			},
			Spec: QuestDBSpec{
				Image: "questdb/questdb:latest",
				Volume: QuestDBVolumeSpec{
					Size: resource.MustParse("1Gi"),
				},
			},
		}

		Expect(k8sClient.Create(ctx, q)).Should(Succeed())

		snap = &QuestDBSnapshot{
			ObjectMeta: metav1.ObjectMeta{
				Name:      q.Name,
				Namespace: "default",
			},
			Spec: QuestDBSnapshotSpec{
				QuestDBName:             q.Name,
				VolumeSnapshotClassName: pointer.String("csi-hostpath-snapclass"),
			},
		}
	})

	Context("When validating QuestDBSnapshot Creates", func() {

		It("should accept the default values", func() {
			Expect(k8sClient.Create(ctx, snap)).To(Succeed())
		})

		It("should reject empty questdb names", func() {
			snap.Spec.QuestDBName = ""
			Expect(k8sClient.Create(ctx, snap)).ToNot(Succeed())
		})

		It("should reject empty volume snapshot class names", func() {
			snap.Spec.VolumeSnapshotClassName = pointer.String("")
			Expect(k8sClient.Create(ctx, snap)).ToNot(Succeed())
		})

		It("should accept nil volume snapshot class names", func() {
			snap.Spec.VolumeSnapshotClassName = nil
			Expect(k8sClient.Create(ctx, snap)).To(Succeed())
		})

	})

	Context("When validating QuestDBSnapshot Updates", func() {

		It("should reject updates to questdb names", func() {
			Expect(k8sClient.Create(ctx, snap)).To(Succeed())
			snap.Spec.QuestDBName = "foo"
			Expect(k8sClient.Update(ctx, snap)).ToNot(Succeed())
		})

		It("should reject updates to volume snapshot class names", func() {
			Expect(k8sClient.Create(ctx, snap)).To(Succeed())
			snap.Spec.VolumeSnapshotClassName = pointer.String("foo")
			Expect(k8sClient.Update(ctx, snap)).ToNot(Succeed())
		})

		It("should reject updates to backoff limit", func() {
			Expect(k8sClient.Create(ctx, snap)).To(Succeed())
			snap.Spec.JobBackoffLimit = 500
			Expect(k8sClient.Update(ctx, snap)).ToNot(Succeed())
		})
	})

})
