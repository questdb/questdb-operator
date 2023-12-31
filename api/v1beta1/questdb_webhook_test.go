package v1beta1

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

var _ = Describe("QuestDB Webhook", func() {
	var (
		q *QuestDB
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
					Size: resource.MustParse("10Gi"),
				},
			},
		}
	})

	Context("When validating QuestDB Creates", func() {

		It("should accept the default values", func() {
			Expect(k8sClient.Create(ctx, q)).To(Succeed())
		})

		It("should reject volume sizes of 0", func() {
			q.Spec.Volume.Size = resource.MustParse("0")
			Expect(k8sClient.Create(ctx, q)).ToNot(Succeed())
		})

		It("should reject reserved config keys", func() {
			q.Spec.Config.ServerConfig = "http.bind.to="
			Expect(k8sClient.Create(ctx, q)).ToNot(Succeed())

			q.Spec.Config.ServerConfig = "line.tcp.net.bind.to="
			Expect(k8sClient.Create(ctx, q)).ToNot(Succeed())

			q.Spec.Config.ServerConfig = "pg.net.bind.to="
			Expect(k8sClient.Create(ctx, q)).ToNot(Succeed())

			q.Spec.Config.ServerConfig = "\notherstuff\nhttp.bind.to="
			Expect(k8sClient.Create(ctx, q)).ToNot(Succeed())

			// Respect commented-out lines
			q.Spec.Config.ServerConfig = "\notherstuff\n#line.tcp.net.bind.to="
			Expect(k8sClient.Create(ctx, q)).To(Succeed())
		})

		It("should reject missing volume size", func() {
			q.Spec.Volume.Size = resource.Quantity{}
			Expect(k8sClient.Create(ctx, q)).ToNot(Succeed())
		})

		It("should not allow selector and snapshotName", func() {
			q.Spec.Volume.SnapshotName = "foo"
			q.Spec.Volume.Selector = &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"foo": "bar",
				},
			}
			Expect(k8sClient.Create(ctx, q)).ToNot(Succeed())
		})

		It("should handle an empty image pull policy", func() {
			q.Spec.ImagePullPolicy = ""
			Expect(k8sClient.Create(ctx, q)).To(Succeed())
			Expect(q.Spec.ImagePullPolicy).To(Equal(v1.PullIfNotPresent))
		})
	})

	Context("When validating QuestDB Updates", func() {

		It("should accept an update with only labels changing", func() {
			q.Spec.Volume.StorageClassName = pointer.String("test-storage-class")
			q.Labels = map[string]string{
				"test-label": "test-value",
			}
			Expect(k8sClient.Create(ctx, q)).To(Succeed())

			q.Labels["another-label"] = "another-value"
			Expect(k8sClient.Update(ctx, q)).To(Succeed())
		})

		It("should reject volume shrinking", func() {
			Expect(k8sClient.Create(ctx, q)).To(Succeed())
			q.Spec.Volume.Size = resource.MustParse("0")
			Expect(k8sClient.Update(ctx, q)).ToNot(Succeed())
		})

		It("should reject changes to the volume name", func() {
			Expect(k8sClient.Create(ctx, q)).To(Succeed())
			q.Spec.Volume.VolumeName = "foo"
			Expect(k8sClient.Update(ctx, q)).ToNot(Succeed())
		})

		It("should reject changes to the storageclass", func() {
			Expect(k8sClient.Create(ctx, q)).To(Succeed())
			q.Spec.Volume.StorageClassName = pointer.String("foo")
			Expect(k8sClient.Update(ctx, q)).ToNot(Succeed())
		})

		It("should reject adding a new volume selector", func() {
			Expect(k8sClient.Create(ctx, q)).To(Succeed())
			q.Spec.Volume.Selector = &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"foo": "bar",
				},
			}
			Expect(k8sClient.Update(ctx, q)).ToNot(Succeed())
		})

		It("should reject changes to the volume selector", func() {
			q.Spec.Volume.Selector = &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"foo": "bar",
				},
			}
			Expect(k8sClient.Create(ctx, q)).To(Succeed())
			q.Spec.Volume.Selector = &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"notfoo": "notbar",
				},
			}
			Expect(k8sClient.Update(ctx, q)).ToNot(Succeed())
		})

		It("should reject changes to the snapshot name", func() {
			Expect(k8sClient.Create(ctx, q)).To(Succeed())
			q.Spec.Volume.SnapshotName = "foo"
			Expect(k8sClient.Update(ctx, q)).ToNot(Succeed())
		})

	})

})
