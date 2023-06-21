package controller

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	crdv1beta1 "github.com/questdb/questdb-operator/api/v1beta1"
	testutils "github.com/questdb/questdb-operator/tests/utils"
	"github.com/thejerf/abtime"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("QuestDBSnapshotSchedule Controller", func() {
	var (
		q     *crdv1beta1.QuestDB
		sched *crdv1beta1.QuestDBSnapshotSchedule

		timeSource *abtime.ManualTime

		timeout = time.Second * 2
		//consistencyTimeout = time.Millisecond * 600
		interval = time.Millisecond * 100

		r *QuestDBSnapshotScheduleReconciler
	)

	Context("golden path case", Ordered, func() {
		var (
			snapList = &crdv1beta1.QuestDBSnapshotList{}
		)

		BeforeAll(func() {
			r = &QuestDBSnapshotScheduleReconciler{
				Client:     k8sClient,
				Scheme:     scheme.Scheme,
				Recorder:   record.NewFakeRecorder(100),
				TimeSource: abtime.NewManual(),
			}

			By("Creating a QuestDB")
			q = testutils.BuildAndCreateMockQuestDB(ctx, k8sClient)

			By("Creating a QuestDBSnapshotSchedule")
			sched = &crdv1beta1.QuestDBSnapshotSchedule{
				ObjectMeta: metav1.ObjectMeta{
					Name:      q.Name,
					Namespace: q.Namespace,
				},
				Spec: crdv1beta1.QuestDBSnapshotScheduleSpec{
					Snapshot: crdv1beta1.QuestDBSnapshotSpec{
						QuestDBName:             q.Name,
						VolumeSnapshotClassName: pointer.String("csi-hostpath-snapclass"),
					},
					Schedule: "*/1 * * * *",
				},
			}

		})

		It("should requeue at the correct time when a schedule is created", func() {

			Expect(k8sClient.Create(ctx, sched)).To(Succeed())
			r.TimeSource = abtime.NewManualAtTime(sched.CreationTimestamp.Time)
			timeSource = r.TimeSource.(*abtime.ManualTime)

			By("Reconciling the QuestDBSnapshotSchedule")
			res, err := r.Reconcile(ctx, ctrl.Request{
				NamespacedName: client.ObjectKeyFromObject(sched),
			})
			Expect(err).ToNot(HaveOccurred())
			nextRunTime := r.TimeSource.Now().Add(time.Minute).Truncate(time.Minute)
			Expect(res.RequeueAfter).To(Equal(nextRunTime.Sub(r.TimeSource.Now())))

		})

		It("should create a snapshot if the cron schedule has triggered", func() {

			By("Bumping the clock more than 1 minute")
			timeSource.Advance(time.Minute + 5*time.Second)

			By("Forcing a reconcile")
			_, err := r.Reconcile(ctx, ctrl.Request{
				NamespacedName: client.ObjectKeyFromObject(sched),
			})
			Expect(err).ToNot(HaveOccurred())

			By("Checking that a snapshot has been created")
			Expect(k8sClient.List(ctx, snapList, client.InNamespace(sched.Namespace))).Should(Succeed())
			Expect(snapList.Items).To(HaveLen(1))
			Expect(snapList.Items[0].OwnerReferences).To(HaveLen(1))
			Expect(snapList.Items[0].OwnerReferences[0].Name).To(Equal(sched.Name))
		})

		It("should report the phase of the latest snapshot", func() {
			By("Getting the latest snapshot")
			snapList := &crdv1beta1.QuestDBSnapshotList{}
			Expect(k8sClient.List(ctx, snapList, client.InNamespace(sched.Namespace))).Should(Succeed())
			Expect(snapList.Items).To(HaveLen(1))

			latestSnap := &snapList.Items[0]

			By("Setting the phase to Succeeded")
			Eventually(func(g Gomega) {
				k8sClient.Get(ctx, client.ObjectKeyFromObject(latestSnap), latestSnap)
				latestSnap.Status.Phase = crdv1beta1.SnapshotSucceeded
				g.Expect(k8sClient.Status().Update(ctx, latestSnap)).To(Succeed())
			}, timeout, interval).Should(Succeed())

			By("Forcing a reconcile")
			_, err := r.Reconcile(ctx, ctrl.Request{
				NamespacedName: client.ObjectKeyFromObject(sched),
			})
			Expect(err).ToNot(HaveOccurred())

			By("Checking that the status has been updated")
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(sched), sched)).To(Succeed())
			Expect(sched.Status.SnapshotPhase).To(Equal(crdv1beta1.SnapshotSucceeded))
		})

		It("should take a second snapshot if the cron schedule has triggered", func() {
			By("Bumping the clock more than 1 minute")
			timeSource.Advance(time.Minute + 5*time.Second)

			By("Forcing a reconcile")
			res, err := r.Reconcile(ctx, ctrl.Request{
				NamespacedName: client.ObjectKeyFromObject(sched),
			})
			Expect(res).To(Equal(ctrl.Result{}))
			Expect(err).ToNot(HaveOccurred())

			By("Checking that a snapshot has been created")
			Eventually(func(g Gomega) {
				snapList := &crdv1beta1.QuestDBSnapshotList{}
				g.Expect(k8sClient.List(ctx, snapList, client.InNamespace(sched.Namespace))).Should(Succeed())
				g.Expect(snapList.Items).To(HaveLen(2))
			}, timeout, interval).Should(Succeed())
		})

		It("should delete the snapshot if the retention policy is set to 1", func() {
			By("Setting the retention policy to 1")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(sched), sched)).To(Succeed())
				sched.Spec.Retention = 1
				g.Expect(k8sClient.Update(ctx, sched)).To(Succeed())
			}, timeout, interval).Should(Succeed())

			By("Setting the status of all snapshots to Succeeded")
			Eventually(func(g Gomega) {

				snapList := &crdv1beta1.QuestDBSnapshotList{}
				g.Expect(k8sClient.List(ctx, snapList, client.InNamespace(sched.Namespace))).Should(Succeed())
				for _, snap := range snapList.Items {
					if snap.Status.Phase != crdv1beta1.SnapshotSucceeded {
						snap.Status.Phase = crdv1beta1.SnapshotSucceeded
						g.Expect(k8sClient.Status().Update(ctx, &snap)).To(Succeed())
					}
				}
			}, timeout, interval).Should(Succeed())

			By("Forcing a reconcile")
			_, err := r.Reconcile(ctx, ctrl.Request{
				NamespacedName: client.ObjectKeyFromObject(sched),
			})
			Expect(err).ToNot(HaveOccurred())

			By("Checking that a snapshot has been deleted")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.List(ctx, snapList, client.InNamespace(sched.Namespace))).Should(Succeed())
				g.Expect(snapList.Items).To(HaveLen(1))
			}, timeout, interval).Should(Succeed())
		})

	})

})
