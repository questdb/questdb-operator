package controller

import (
	"fmt"
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
		timeout  = time.Second * 2
		interval = time.Millisecond * 100
	)

	Context("golden path case", Ordered, func() {
		var (
			snapList   = &crdv1beta1.QuestDBSnapshotList{}
			timeSource *abtime.ManualTime

			q     *crdv1beta1.QuestDB
			sched *crdv1beta1.QuestDBSnapshotSchedule
			r     *QuestDBSnapshotScheduleReconciler
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

			By("Creating a QuestDBSnapshotSchedule that triggers every minute")
			sched = &crdv1beta1.QuestDBSnapshotSchedule{
				ObjectMeta: metav1.ObjectMeta{
					Name:      q.Name,
					Namespace: q.Namespace,
				},
				Spec: crdv1beta1.QuestDBSnapshotScheduleSpec{
					Snapshot: crdv1beta1.QuestDBSnapshotSpec{
						QuestDBName:             q.Name,
						VolumeSnapshotClassName: pointer.String(testutils.SnapshotClassName),
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

			By("Advancing to the to the next minute to trigger a snapshot")
			advanceToTheNextMinute(timeSource)

			By("Forcing a reconcile")
			_, err := r.Reconcile(ctx, ctrl.Request{
				NamespacedName: client.ObjectKeyFromObject(sched),
			})
			Expect(err).ToNot(HaveOccurred())

			By("Checking that a snapshot has been created")
			Expect(k8sClient.List(ctx, snapList, client.InNamespace(sched.Namespace))).Should(Succeed())
			Expect(snapList.Items).To(HaveLen(1))
			Expect(metav1.IsControlledBy(&snapList.Items[0], sched))
		})

		It("should report the phase of the latest snapshot", func() {
			By("Getting the latest snapshot")
			snap := &crdv1beta1.QuestDBSnapshot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      sched.Name + "-" + timeSource.Now().Format("20060102150405"),
					Namespace: sched.Namespace,
				},
			}

			By("Waiting for the snapshot to become pending (since the snapshot controller is running in the background)")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(snap), snap)).To(Succeed())
				g.Expect(snap.Status.Phase).To(Equal(crdv1beta1.SnapshotPending))
			}, timeout, interval).Should(Succeed())

			By("Setting the snapshot to succeeded")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(snap), snap)).To(Succeed())
				snap.Status.Phase = crdv1beta1.SnapshotSucceeded
				Expect(k8sClient.Status().Update(ctx, snap)).To(Succeed())
			}, timeout, interval).Should(Succeed())

			By("Advancing time a few milliseconds")
			advanceTime(timeSource, time.Millisecond*2)

			By("Forcing a reconcile")
			_, err := r.Reconcile(ctx, ctrl.Request{
				NamespacedName: client.ObjectKeyFromObject(sched),
			})
			Expect(err).ToNot(HaveOccurred())

			By("Checking that the schedule status has been updated")
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(sched), sched)).To(Succeed())
			Expect(sched.Status.SnapshotPhase).To(Equal(crdv1beta1.SnapshotSucceeded))
		})

		It("should take a second snapshot if the cron schedule has triggered", func() {
			By("Advancing the clock to the next minute")
			advanceToTheNextMinute(timeSource)

			By("Forcing a reconcile")
			_, err := r.Reconcile(ctx, ctrl.Request{
				NamespacedName: client.ObjectKeyFromObject(sched),
			})
			Expect(err).ToNot(HaveOccurred())

			By("Checking that a snapshot has been created")
			Eventually(func(g Gomega) {
				snap := &crdv1beta1.QuestDBSnapshot{
					ObjectMeta: metav1.ObjectMeta{
						Name:      sched.Name + "-" + timeSource.Now().Format("20060102150405"),
						Namespace: sched.Namespace,
					},
				}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(snap), snap)).To(Succeed())
			}, timeout, interval).Should(Succeed())
		})

		It("should delete the snapshot if the retention policy is set to 1", func() {
			By("Waiting for the snapshot to become pending")
			latestSnap := &crdv1beta1.QuestDBSnapshot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      sched.Name + "-" + timeSource.Now().Format("20060102150405"),
					Namespace: sched.Namespace,
				},
			}
			Eventually(func(g Gomega) {
				k8sClient.Get(ctx, client.ObjectKeyFromObject(latestSnap), latestSnap)
				g.Expect(latestSnap.Status.Phase).To(Equal(crdv1beta1.SnapshotPending))
			}, timeout, interval).Should(Succeed())

			By("Setting the retention policy to 1")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(sched), sched)).To(Succeed())
				sched.Spec.Retention = 1
				g.Expect(k8sClient.Update(ctx, sched)).To(Succeed())
			}, timeout, interval).Should(Succeed())

			By("Setting the snapshot to succeeded")
			latestSnap.Status.Phase = crdv1beta1.SnapshotSucceeded
			Expect(k8sClient.Status().Update(ctx, latestSnap)).To(Succeed())

			By("Advancing time a few milliseconds to avoid creating a new snapshot")
			advanceTime(timeSource, 5*time.Millisecond)

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

	It("should only report the status of snapshots owned by it", func() {
		var (
			sched *crdv1beta1.QuestDBSnapshotSchedule
			r     *QuestDBSnapshotScheduleReconciler
			q     *crdv1beta1.QuestDB
		)

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
					VolumeSnapshotClassName: pointer.String(testutils.SnapshotClassName),
				},
				Schedule: "*/1 * * * *",
			},
		}
		Expect(k8sClient.Create(ctx, sched)).To(Succeed())

		By("Creating a one-off snapshot")
		snap := &crdv1beta1.QuestDBSnapshot{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "one-off-snap",
				Namespace: q.Namespace,
			},
			Spec: crdv1beta1.QuestDBSnapshotSpec{
				QuestDBName: q.Name,
			},
		}
		Expect(k8sClient.Create(ctx, snap)).To(Succeed())

		By("Advancing time at least a minute to trigger a reconcile")
		advanceTime(r.TimeSource.(*abtime.ManualTime), time.Minute+5*time.Second)

		By("Forcing a reconcile")
		_, err := r.Reconcile(ctx, ctrl.Request{
			NamespacedName: client.ObjectKeyFromObject(sched),
		})
		Expect(err).ToNot(HaveOccurred())

		By("Ensuring that the status of the one-off snapshot status is not reported")
		Eventually(func(g Gomega) {
			// Ensure that the one-off snapshot is pending
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(snap), snap)).To(Succeed())
			g.Expect(snap.Status.Phase).To(Equal(crdv1beta1.SnapshotPending))
		}, timeout, interval).Should(Succeed())

		// Now ensure that the schedule status has not updated
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(sched), sched)).To(Succeed())
		Expect(sched.Status.SnapshotPhase).To(Equal(crdv1beta1.QuestDBSnapshotPhase("")))
	})

	It("Should only garbage collect succeeded snapshots", func() {
		var (
			retention  int32 = 5
			sched      *crdv1beta1.QuestDBSnapshotSchedule
			r          *QuestDBSnapshotScheduleReconciler
			q          *crdv1beta1.QuestDB
			timeSource *abtime.ManualTime
		)

		r = &QuestDBSnapshotScheduleReconciler{
			Client:     k8sClient,
			Scheme:     scheme.Scheme,
			Recorder:   record.NewFakeRecorder(100),
			TimeSource: abtime.NewManual(),
		}

		By("Creating a QuestDB")
		q = testutils.BuildAndCreateMockQuestDB(ctx, k8sClient)

		By("Creating a QuestDBSnapshotSchedule with a nonexistant snapshot class to fail all snapshots")
		sched = &crdv1beta1.QuestDBSnapshotSchedule{
			ObjectMeta: metav1.ObjectMeta{
				Name:      q.Name,
				Namespace: q.Namespace,
			},
			Spec: crdv1beta1.QuestDBSnapshotScheduleSpec{
				Snapshot: crdv1beta1.QuestDBSnapshotSpec{
					QuestDBName:             q.Name,
					VolumeSnapshotClassName: pointer.String("this-snapshot-class-does-not-exist"),
				},
				Schedule:  "*/1 * * * *",
				Retention: retention,
			},
		}
		Expect(k8sClient.Create(ctx, sched)).To(Succeed())
		r.TimeSource = abtime.NewManualAtTime(sched.CreationTimestamp.Time)
		timeSource = r.TimeSource.(*abtime.ManualTime)

		By("Advancing to the next minute for determinism")
		advanceToTheNextMinute(timeSource)
		testDebugLog.Info(timeSource.Now().Format(time.RFC3339Nano))

		By("Advancing time by minute enough to create retention * 2 snapshots")
		for i := int32(0); i < retention*2; i++ {

			advanceTime(timeSource, time.Minute)
			testDebugLog.Info(timeSource.Now().Format(time.RFC3339Nano))
			_, err := r.Reconcile(ctx, ctrl.Request{
				NamespacedName: client.ObjectKeyFromObject(sched),
			})
			Expect(err).ToNot(HaveOccurred())

			// Wait for the snapshot to transition to Failed before advancing time again
			snap := &crdv1beta1.QuestDBSnapshot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      sched.Name + "-" + r.TimeSource.Now().Format("20060102150405"),
					Namespace: sched.Namespace,
				},
			}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(snap), snap)).To(Succeed())
				g.Expect(snap.Status.Phase).To(Equal(crdv1beta1.SnapshotFailed))
			}, timeout, interval).Should(Succeed())

		}

		By("Getting all snapshots and ensuring that they are all failed")
		// We need to migrate snapshots from Pending to either Succeeded or Failed for the reconciler
		// to create a new snapshot. Since we want to test retention of failed
		snapList := &crdv1beta1.QuestDBSnapshotList{}
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.List(ctx, snapList, client.InNamespace(sched.Namespace))).Should(Succeed())
			g.Expect(snapList.Items).To(HaveLen(int(retention * 2)))
			for _, snap := range snapList.Items {
				g.Expect(snap.Status.Phase).To(Equal(crdv1beta1.SnapshotFailed))
			}
		}, timeout, interval).Should(Succeed())

		By("Set retention + 1 snapshots to succeeded and delete their finalizers")
		for idx := range snapList.Items {
			if idx < int(retention+1) {
				Eventually(func(g Gomega) {
					g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(&snapList.Items[idx]), &snapList.Items[idx])).To(Succeed())
					snapList.Items[idx].Status.Phase = crdv1beta1.SnapshotSucceeded
					snapList.Items[idx].Finalizers = []string{}
					g.Expect(k8sClient.Status().Update(ctx, &snapList.Items[idx])).To(Succeed())
				}, timeout, interval).Should(Succeed())
			}
		}

		By("Advancing time a small amount to not trigger another snapshot creation")
		advanceTime(r.TimeSource.(*abtime.ManualTime), 5*time.Millisecond)
		testDebugLog.Info(timeSource.Now().Format(time.RFC3339Nano))

		By("Forcing a reconcile")
		_, err := r.Reconcile(ctx, ctrl.Request{
			NamespacedName: client.ObjectKeyFromObject(sched),
		})
		Expect(err).ToNot(HaveOccurred())

		By("Checking that the list has shrunk by 1")
		Expect(k8sClient.List(ctx, snapList, client.InNamespace(sched.Namespace))).Should(Succeed())
		succeeded := []crdv1beta1.QuestDBSnapshot{}
		failed := []crdv1beta1.QuestDBSnapshot{}
		for _, snap := range snapList.Items {
			switch snap.Status.Phase {
			case crdv1beta1.SnapshotSucceeded:
				succeeded = append(succeeded, snap)
			case crdv1beta1.SnapshotFailed:
				failed = append(failed, snap)
			default:
				Expect(false).To(Equal(true), fmt.Sprintf("no snapshot should have phase %q", snap.Status.Phase))
			}
		}
		Expect(succeeded).Should(HaveLen(int(retention)))
		Expect(failed).Should(HaveLen(int(retention - 1)))
		Expect(snapList.Items).To(HaveLen(int(retention*2 - 1)))

	})

})

func advanceToTheNextMinute(timeSource *abtime.ManualTime) {
	now := timeSource.Now()
	nextMinute := now.Add(time.Minute).Truncate(time.Minute)
	timeToNextMinute := nextMinute.Sub(timeSource.Now())
	timeSource.Advance(timeToNextMinute)
	testDebugLog.Info("Advanced Time To Next Minute", map[string]string{
		"oldTime":    now.Format(time.RFC3339Nano),
		"nextMinute": nextMinute.Format(time.RFC3339Nano),
		"newTime":    timeSource.Now().Format(time.RFC3339Nano),
	})
}

func advanceTime(timeSource *abtime.ManualTime, d time.Duration) {
	now := timeSource.Now()
	timeSource.Advance(d)
	testDebugLog.Info("Advanced Time", map[string]string{
		"oldTime":  now.Format(time.RFC3339Nano),
		"duration": d.String(),
		"newTime":  timeSource.Now().Format(time.RFC3339Nano),
	})
}
