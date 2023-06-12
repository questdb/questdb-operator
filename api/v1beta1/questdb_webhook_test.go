package v1beta1

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
					Size: resource.MustParse("1Gi"),
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
			q.Spec.Config.DbConfig = "http.bind.to="
			Expect(k8sClient.Create(ctx, q)).ToNot(Succeed())

			q.Spec.Config.DbConfig = "line.tcp.net.bind.to="
			Expect(k8sClient.Create(ctx, q)).ToNot(Succeed())

			q.Spec.Config.DbConfig = "pg.net.bind.to="
			Expect(k8sClient.Create(ctx, q)).ToNot(Succeed())

			q.Spec.Config.DbConfig = "\notherstuff\nhttp.bind.to="
			Expect(k8sClient.Create(ctx, q)).ToNot(Succeed())

			q.Spec.Config.DbConfig = "\notherstuff\n#line.tcp.net.bind.to="
			Expect(k8sClient.Create(ctx, q)).To(Succeed())
		})
	})

	Context("When validating QuestDB Updates", func() {

		It("should reject volume shrinking", func() {
			Expect(k8sClient.Create(ctx, q)).To(Succeed())
			q.Spec.Volume.Size = resource.MustParse("0")
			Expect(k8sClient.Update(ctx, q)).ToNot(Succeed())
		})
	})

})
