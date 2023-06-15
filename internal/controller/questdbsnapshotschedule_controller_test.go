package controller

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	crdv1beta1 "github.com/questdb/questdb-operator/api/v1beta1"
	testutils "github.com/questdb/questdb-operator/tests/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("QuestDBSnapshotSchedule Controller", func() {
	var (
		q     *crdv1beta1.QuestDB
		sched *crdv1beta1.QuestDBSnapshotSchedule

		timeout = time.Second * 2
		//consistencyTimeout = time.Millisecond * 600
		interval = time.Millisecond * 100
	)

	BeforeEach(func() {

		q = testutils.BuildMockQuestDB(ctx, k8sClient)

		sched = &crdv1beta1.QuestDBSnapshotSchedule{
			ObjectMeta: metav1.ObjectMeta{
				Name:      q.Name,
				Namespace: q.Namespace,
			},
			Spec: crdv1beta1.QuestDBSnapshotScheduleSpec{
				Snapshot: crdv1beta1.QuestDBSnapshotSpec{
					QuestDBName:             q.Name,
					VolumeSnapshotClassName: "csi-hostpath-snapclass",
				},
				Schedule: "*/1 * * * *",
			},
		}
	})

	Context("golden path case", Ordered, func() {
		var (
			origSched crdv1beta1.QuestDBSnapshotSchedule

			snapList = &crdv1beta1.QuestDBSnapshotList{}
		)

		BeforeAll(func() {
			By("Creating the QuestDBSnapshotSchedule")
			Expect(k8sClient.Create(ctx, sched)).To(Succeed())
			origSched = *sched
		})

		It("should create a snapshot if the cron schedule has triggered", func() {

			By("Bumping the clock 1 minute")
			timeSource.Advance(time.Minute + time.Second)

			By("Checking that a snapshot has been created")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.List(ctx, snapList, client.InNamespace(origSched.Namespace))).Should(Succeed())
				g.Expect(snapList.Items).To(HaveLen(1))
				g.Expect(snapList.Items[0].OwnerReferences).To(HaveLen(1))
				g.Expect(snapList.Items[0].OwnerReferences[0].Name).To(Equal(origSched.Name))
			}, timeout, interval).Should(Succeed())
		})

		It("should report the phase of the latest snapshot", func() {
			By("Getting the latest snapshot")
			snapList := &crdv1beta1.QuestDBSnapshotList{}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.List(ctx, snapList, client.InNamespace(origSched.Namespace))).Should(Succeed())
				g.Expect(snapList.Items).To(HaveLen(1))
			}, timeout, interval).Should(Succeed())

			latestSnap := &snapList.Items[0]

			By("Setting the phase to Succeeded")
			latestSnap.Status.Phase = crdv1beta1.SnapshotSucceeded
			Expect(k8sClient.Status().Update(ctx, latestSnap)).To(Succeed())

			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: origSched.Name, Namespace: origSched.Namespace}, &origSched)).To(Succeed())
				g.Expect(origSched.Status.SnapshotPhase).To(Equal(crdv1beta1.SnapshotSucceeded))
			}, timeout, interval).Should(Succeed())
		})

	})

})
