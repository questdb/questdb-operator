package v1beta1

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/resource"

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
				VolumeSnapshotClassName: "csi-hostpath-snapclass",
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
			snap.Spec.VolumeSnapshotClassName = ""
			Expect(k8sClient.Create(ctx, snap)).ToNot(Succeed())
		})

	})

	Context("When validating QuestDB Updates", func() {
	})

})