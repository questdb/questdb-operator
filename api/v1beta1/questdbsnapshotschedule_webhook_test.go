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

var _ = Describe("QuestDBSnapshotSchedule Webhook", func() {
	var (
		q     *QuestDB
		sched *QuestDBSnapshotSchedule
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

		sched = &QuestDBSnapshotSchedule{
			ObjectMeta: metav1.ObjectMeta{
				Name:      q.Name,
				Namespace: "default",
			},
			Spec: QuestDBSnapshotScheduleSpec{
				Snapshot: QuestDBSnapshotSpec{
					QuestDBName:             q.Name,
					VolumeSnapshotClassName: pointer.String("snapclass"),
				},
				Schedule: "*/1 * * * *",
			},
		}

	})

	Context("When validating QuestDBSnapshotSchedule Creates", func() {

		It("should accept the default values", func() {
			Expect(k8sClient.Create(ctx, sched)).To(Succeed())
		})

		It("should reject invalid cron schedules", func() {
			sched.Spec.Schedule = "foo"
			Expect(k8sClient.Create(ctx, sched)).ToNot(Succeed())
		})

		It("should reject empty questdb names", func() {
			sched.Spec.Snapshot.QuestDBName = ""
			Expect(k8sClient.Create(ctx, sched)).ToNot(Succeed())
		})

		It("should reject empty volume snapshot class names", func() {
			sched.Spec.Snapshot.VolumeSnapshotClassName = pointer.String("")
			Expect(k8sClient.Create(ctx, sched)).ToNot(Succeed())
		})

	})

	Context("When validating QuestDBSnapshotSchedule Updates", func() {

		It("should reject invalid cron schedules", func() {
			Expect(k8sClient.Create(ctx, sched)).To(Succeed())
			sched.Spec.Schedule = "foo"
			Expect(k8sClient.Update(ctx, sched)).ToNot(Succeed())
		})

		It("should reject updates to questdb names", func() {
			Expect(k8sClient.Create(ctx, sched)).To(Succeed())
			sched.Spec.Snapshot.QuestDBName = "foo"
			Expect(k8sClient.Update(ctx, sched)).ToNot(Succeed())
		})

		It("should reject updates to volume snapshot class names", func() {
			Expect(k8sClient.Create(ctx, sched)).To(Succeed())
			sched.Spec.Snapshot.VolumeSnapshotClassName = pointer.String("foo")
			Expect(k8sClient.Update(ctx, sched)).ToNot(Succeed())
		})

		It("should reject updates to backoff limit", func() {
			Expect(k8sClient.Create(ctx, sched)).To(Succeed())
			sched.Spec.Snapshot.JobBackoffLimit = 500
			Expect(k8sClient.Update(ctx, sched)).ToNot(Succeed())
		})
	})

})
