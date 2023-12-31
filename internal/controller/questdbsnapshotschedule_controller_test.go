package controller

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	crdv1beta1 "github.com/questdb/questdb-operator/api/v1beta1"
	testutils "github.com/questdb/questdb-operator/tests/utils"
	"github.com/thejerf/abtime"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("QuestDBSnapshotSchedule Controller", func() {
	var (
		timeout  = time.Second * 2
		interval = time.Millisecond * 100

		recorder = record.NewFakeRecorder(100)
	)

	Context("golden path case", Ordered, func() {
		var (
			timeSource *abtime.ManualTime

			q     *crdv1beta1.QuestDB
			sched *crdv1beta1.QuestDBSnapshotSchedule
			r     *QuestDBSnapshotScheduleReconciler
		)

		BeforeAll(func() {
			r = &QuestDBSnapshotScheduleReconciler{
				Client:     k8sClient,
				Scheme:     scheme.Scheme,
				Recorder:   recorder,
				TimeSource: abtime.NewManual(),
			}

			By("Creating a QuestDB")
			q = testutils.BuildAndCreateMockQuestDB(ctx, k8sClient)

			By("Creating a QuestDBSnapshotSchedule that triggers every minute")
			// These should reliably fail so we can reliably trigger more snapshots
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
					Schedule: "*/1 * * * *",
				},
			}

		})

		It("should create a snapshot at the next minute, and another after that", func() {
			snapList := &crdv1beta1.QuestDBSnapshotList{}

			Expect(k8sClient.Create(ctx, sched)).To(Succeed())
			r.TimeSource = abtime.NewManualAtTime(sched.CreationTimestamp.Time)
			timeSource = r.TimeSource.(*abtime.ManualTime)

			advanceToTheNextMinute(timeSource)

			By("Reconciling the QuestDBSnapshotSchedule")
			_, err := reconcileSnapshotSchedules(ctx, r, ctrl.Request{
				NamespacedName: client.ObjectKeyFromObject(sched),
			})
			Expect(err).ToNot(HaveOccurred())

			By("It should have created a snapshot")
			Expect(k8sClient.List(ctx, snapList, client.InNamespace(sched.Namespace))).Should(Succeed())
			Expect(snapList.Items).Should(HaveLen(1))

			By("Waiting for the snapshot to fail")
			Eventually(func(g Gomega) {
				snap := snapList.Items[0]
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(&snap), &snap)).Should(Succeed())
				g.Expect(snap.Status.Phase).Should(Equal(crdv1beta1.SnapshotFailed))
			}, timeout, interval).Should(Succeed())

			By("Advance less than a minute and see that there is still only 1 snapshot")
			advanceTime(timeSource, time.Second*1)
			_, err = reconcileSnapshotSchedules(ctx, r, ctrl.Request{
				NamespacedName: client.ObjectKeyFromObject(sched),
			})
			Expect(err).ToNot(HaveOccurred())

			By("No new snapshot should have been created")
			Expect(k8sClient.List(ctx, snapList, client.InNamespace(sched.Namespace))).Should(Succeed())
			Expect(snapList.Items).Should(HaveLen(1))

			By("Now advance time to the next minute and see that a new snapshot is created")
			advanceToTheNextMinute(timeSource)
			_, err = reconcileSnapshotSchedules(ctx, r, ctrl.Request{
				NamespacedName: client.ObjectKeyFromObject(sched),
			})
			Expect(err).ToNot(HaveOccurred())

			By("A new snapshot should have been created")
			Expect(k8sClient.List(ctx, snapList, client.InNamespace(sched.Namespace))).Should(Succeed())
			Expect(snapList.Items).Should(HaveLen(2))

		})

		It("should report the phase of the latest snapshot", func() {
			By("Getting the latest snapshot by using the current frozen time")
			snap := &crdv1beta1.QuestDBSnapshot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      sched.Name + "-" + timeSource.Now().Format("20060102150405"),
					Namespace: sched.Namespace,
				},
			}

			By("Waiting for the snapshot to fail")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(snap), snap)).To(Succeed())
				g.Expect(snap.Status.Phase).To(Equal(crdv1beta1.SnapshotFailed))
			}, timeout, interval).Should(Succeed())

			By("Setting the snapshot to succeeded (different status than the first snapshot, which is Failed)")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(snap), snap)).To(Succeed())
				snap.Status.Phase = crdv1beta1.SnapshotSucceeded
				g.Expect(k8sClient.Status().Update(ctx, snap)).Should(Succeed())
			}, timeout, interval).Should(Succeed())

			By("Advancing time a few milliseconds to prevent another snapshot from being created in the next reconcile")
			advanceTime(timeSource, time.Millisecond*5)

			By("Reconciling the schedule")
			_, err := reconcileSnapshotSchedules(ctx, r, ctrl.Request{
				NamespacedName: client.ObjectKeyFromObject(sched),
			})
			Expect(err).ToNot(HaveOccurred())

			By("Checking that the schedule status has been updated to match the latest snapshot")
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(sched), sched)).To(Succeed())
			Expect(sched.Status.SnapshotPhase).To(Equal(crdv1beta1.SnapshotSucceeded))
		})

		It("should delete the second snapshot if the retention policy is set to 1", func() {
			snapList := &crdv1beta1.QuestDBSnapshotList{}

			By("Setting the retention policy to 1")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(sched), sched)).To(Succeed())
				sched.Spec.Retention = 1
				g.Expect(k8sClient.Update(ctx, sched)).To(Succeed())
			}, timeout, interval).Should(Succeed())

			By("Setting all snapshots to succeeded (keeping their finalizers)")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.List(ctx, snapList, client.InNamespace(sched.Namespace))).Should(Succeed())
				g.Expect(snapList.Items).Should(HaveLen(2))
				for _, snap := range snapList.Items {
					snap.Status.Phase = crdv1beta1.SnapshotSucceeded
					g.Expect(k8sClient.Status().Update(ctx, &snap)).Should(Succeed())
				}
			}, timeout, interval).Should(Succeed())

			By("Advancing time a few milliseconds to avoid creating a new snapshot on reconcile")
			advanceTime(timeSource, 5*time.Millisecond)

			By("Forcing a reconcile")
			_, err := reconcileSnapshotSchedules(ctx, r, ctrl.Request{
				NamespacedName: client.ObjectKeyFromObject(sched),
			})
			Expect(err).ToNot(HaveOccurred())

			By("Checking that a single snapshot has been marked for deletion")
			Expect(k8sClient.List(ctx, snapList, client.InNamespace(sched.Namespace))).Should(Succeed())
			var foundDeletedSnap bool
			for _, snap := range snapList.Items {
				if snap.DeletionTimestamp != nil {
					Expect(foundDeletedSnap).Should(BeFalse())
					Expect(snap.Status.Phase).Should(Equal(crdv1beta1.SnapshotSucceeded))
					foundDeletedSnap = true
				}
			}
		})

	})

	It("should only report the status of snapshots owned by it", func() {
		var (
			sched *crdv1beta1.QuestDBSnapshotSchedule
			r     *QuestDBSnapshotScheduleReconciler
			q     *crdv1beta1.QuestDB

			timeSource *abtime.ManualTime
		)

		r = &QuestDBSnapshotScheduleReconciler{
			Client:     k8sClient,
			Scheme:     scheme.Scheme,
			Recorder:   recorder,
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
		r.TimeSource = abtime.NewManualAtTime(sched.CreationTimestamp.Time)
		timeSource = r.TimeSource.(*abtime.ManualTime)

		By("Creating a one-off snapshot outside of the controller scope")
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

		By("Ensure that the one-off snapshot has transitioned to pending")
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(snap), snap)).To(Succeed())
			g.Expect(snap.Status.Phase).To(Equal(crdv1beta1.SnapshotPending))
		}, timeout, interval).Should(Succeed())

		By("Failing the one-off snapshot")
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(snap), snap)).To(Succeed())
			snap.Status.Phase = crdv1beta1.SnapshotFailed
			g.Expect(k8sClient.Update(ctx, snap)).Should(Succeed())
		}, timeout, interval).Should(Succeed())

		By("Advancing time just before the next minute to not create a new snapshot on reconcile")
		advanceJustBeforeTheNextMinute(timeSource)

		By("Forcing a reconcile")
		_, err := reconcileSnapshotSchedules(ctx, r, ctrl.Request{
			NamespacedName: client.ObjectKeyFromObject(sched),
		})
		Expect(err).ToNot(HaveOccurred())

		By("Ensuring that the schedule status has not updated since it has created no snapshot")
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
			Recorder:   recorder,
			TimeSource: abtime.NewManual(),
		}

		By("Creating a QuestDB")
		q = testutils.BuildAndCreateMockQuestDB(ctx, k8sClient)

		By("Creating a QuestDBSnapshotSchedule with a nonexistant snapshot class to fail all snapshots")
		// Failing all snapshots will keep the finalizer, so no snapshots will actually get deleted later on in the test
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

		By("Advancing to the next minute for deterministic test results")
		advanceToTheNextMinute(timeSource)

		By("Advancing time by a minute enough times to create retention * 2 snapshots")
		for i := int32(0); i < retention*2; i++ {

			advanceTime(timeSource, time.Minute)
			_, err := reconcileSnapshotSchedules(ctx, r, ctrl.Request{
				NamespacedName: client.ObjectKeyFromObject(sched),
			})
			Expect(err).ToNot(HaveOccurred())

			By("Waiting for the newly created snapshot to transition to Failed before continuing the loop")
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
		snapList := &crdv1beta1.QuestDBSnapshotList{}
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.List(ctx, snapList, client.InNamespace(sched.Namespace))).Should(Succeed())
			g.Expect(snapList.Items).To(HaveLen(int(retention * 2)))
			for _, snap := range snapList.Items {
				g.Expect(snap.Status.Phase).To(Equal(crdv1beta1.SnapshotFailed))
			}
		}, timeout, interval).Should(Succeed())

		By("Set (retention + 1) snapshots to succeeded, which should force the reconciler to delete one of them")
		for idx := range snapList.Items {
			if idx < int(retention+1) {
				Eventually(func(g Gomega) {
					g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(&snapList.Items[idx]), &snapList.Items[idx])).To(Succeed())

					snapList.Items[idx].Status.Phase = crdv1beta1.SnapshotSucceeded
					g.Expect(k8sClient.Status().Update(ctx, &snapList.Items[idx])).To(Succeed())

				}, timeout, interval).Should(Succeed())
			}
		}

		By("Advancing time a small amount to not trigger another snapshot creation")
		advanceTime(r.TimeSource.(*abtime.ManualTime), 5*time.Millisecond)

		By("Forcing a reconcile")
		_, err := reconcileSnapshotSchedules(ctx, r, ctrl.Request{
			NamespacedName: client.ObjectKeyFromObject(sched),
		})
		Expect(err).ToNot(HaveOccurred())

		By("Checking that one snapshot has a status of succeeded and a non-nil deletion timestamp")
		// Since they all still have finalizers, the snapshot won't actually be deleted
		Expect(k8sClient.List(ctx, snapList, client.InNamespace(sched.Namespace))).Should(Succeed())
		var foundDeletedSnap bool
		for _, snap := range snapList.Items {
			if snap.DeletionTimestamp != nil {
				Expect(foundDeletedSnap).Should(BeFalse())
				Expect(snap.Status.Phase).Should(Equal(crdv1beta1.SnapshotSucceeded))
				foundDeletedSnap = true
			}
		}
	})

	Context("Requeue and LastSnapshotTime tests", func() {
		cases := []map[string]any{
			{
				"description":             "Given a 1x a day schedule, it should trigger a snapshot if exactly 24 hours has passed since the last snapshot",
				"cron":                    "0 0 * * *",
				"currentTime":             "2023-01-02 00:00:00",
				"lastSnapshotTime":        "2023-01-01 00:00:00",
				"expectedRequeueDuration": time.Hour * 24,
				"expectSnapshotCreated":   true,
			},
			{
				"description":             "Given a 1x a day schedule, it should trigger a snapshot 24 hours + 1 second have elapsed",
				"cron":                    "0 0 * * *",
				"currentTime":             "2023-01-02 00:00:01",
				"lastSnapshotTime":        "2023-01-01 00:00:00",
				"expectedRequeueDuration": time.Hour*24 - time.Second,
				"expectSnapshotCreated":   true,
			},
			{
				"description":             "Given a 1x a day schedule, it should not trigger a snapshot if a day has not elapsed since the last snapshot",
				"cron":                    "0 0 * * *",
				"currentTime":             "2023-01-01 12:00:00",
				"lastSnapshotTime":        "2023-01-01 00:00:00",
				"expectedRequeueDuration": time.Hour * 12,
				"expectSnapshotCreated":   false,
			},
		}

		for _, c := range cases {
			c := c

			It(c["description"].(string), func() {

				currentTime, err := time.ParseInLocation(time.DateTime, c["currentTime"].(string), time.Local)
				Expect(err).Should(Succeed())
				lastSnapshotTime, err := time.ParseInLocation(time.DateTime, c["lastSnapshotTime"].(string), time.Local)
				Expect(err).Should(Succeed())

				sched := &crdv1beta1.QuestDBSnapshotSchedule{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-sched",
						Namespace: fmt.Sprintf("test-%d", time.Now().UnixNano()),
					},
					Spec: crdv1beta1.QuestDBSnapshotScheduleSpec{
						Schedule: c["cron"].(string),
						Snapshot: crdv1beta1.QuestDBSnapshotSpec{
							QuestDBName: "test-qdb",
						},
					},
				}

				By("Creating the namespace")
				n := &v1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: sched.Namespace,
					},
				}
				Expect(k8sClient.Create(ctx, n)).Should(Succeed())

				By("Creating the schedule")
				Expect(k8sClient.Create(ctx, sched)).Should(Succeed())

				By("Updating the schedule's status")
				sched.Status = crdv1beta1.QuestDBSnapshotScheduleStatus{
					LastSnapshot: metav1.NewTime(lastSnapshotTime),
				}
				Expect(k8sClient.Status().Update(ctx, sched)).To(Succeed())

				r := QuestDBSnapshotScheduleReconciler{
					Client:     k8sClient,
					Scheme:     scheme.Scheme,
					Recorder:   recorder,
					TimeSource: abtime.NewManualAtTime(currentTime),
				}

				res, err := reconcileSnapshotSchedules(ctx, &r, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(sched)})
				Expect(err).To(Succeed())
				Expect(res.RequeueAfter).To(Equal(c["expectedRequeueDuration"]))

				snapList := &crdv1beta1.QuestDBSnapshotList{}
				Expect(k8sClient.List(ctx, snapList, client.InNamespace(sched.Namespace))).To(Succeed())
				var expectedLen int
				if c["expectSnapshotCreated"].(bool) {
					expectedLen++
				}
				Expect(snapList.Items).To(HaveLen(expectedLen))

			})

		}

	})

})

func advanceToTheNextMinute(timeSource *abtime.ManualTime) {
	now := timeSource.Now()
	nextMinute := now.Add(time.Minute).Truncate(time.Minute)
	timeToNextMinute := nextMinute.Sub(timeSource.Now())
	timeSource.Advance(timeToNextMinute)
	testDebugLog.Info("Advanced Time To Next Minute",
		"oldTime", now.Format(time.RFC3339Nano),
		"nextMinute", nextMinute.Format(time.RFC3339Nano),
		"newTime", timeSource.Now().Format(time.RFC3339Nano),
	)
}

func advanceJustBeforeTheNextMinute(timeSource *abtime.ManualTime) {
	now := timeSource.Now()
	nextMinute := now.Add(time.Minute).Truncate(time.Minute)
	timeToJustBeforeTheNextMinute := nextMinute.Sub(timeSource.Now()) - time.Millisecond
	timeSource.Advance(timeToJustBeforeTheNextMinute)
	testDebugLog.Info("Advanced Time Right Before The Next Minute",
		"oldTime", now.Format(time.RFC3339Nano),
		"nextMinute", nextMinute.Format(time.RFC3339Nano),
		"newTime", timeSource.Now().Format(time.RFC3339Nano),
	)
}

func advanceTime(timeSource *abtime.ManualTime, d time.Duration) {
	now := timeSource.Now()
	timeSource.Advance(d)
	testDebugLog.Info("Advanced Time",
		"oldTime", now.Format(time.RFC3339Nano),
		"duration", d.String(),
		"newTime", timeSource.Now().Format(time.RFC3339Nano),
	)
}

func reconcileSnapshotSchedules(ctx context.Context, r *QuestDBSnapshotScheduleReconciler, req reconcile.Request) (reconcile.Result, error) {
	testDebugLog.Info("Reconciling", "resource", req.String())
	res, err := r.Reconcile(ctx, req)

	// Flush the event buffer synchronously so we can accurately determine when things are happening
	var flushed bool
	for !flushed {
		select {
		case e := <-r.Recorder.(*record.FakeRecorder).Events:
			testDebugLog.Info(e, "Reconciler", "QuestDBSnapshotSchedule")
		default:
			flushed = true
		}
	}
	return res, err
}
