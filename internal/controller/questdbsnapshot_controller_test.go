package controller

import (
	"fmt"
	"time"

	volumesnapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	crdv1beta1 "github.com/questdb/questdb-operator/api/v1beta1"
	testutils "github.com/questdb/questdb-operator/tests/utils"
)

var _ = Describe("QuestDBSnapshot Controller", func() {
	var (
		timeout            = time.Second * 2
		consistencyTimeout = time.Millisecond * 500
		interval           = time.Millisecond * 100
	)

	Context("When a QuestDBSnapshot is created (golden path)", Ordered, func() {
		var (
			q    *crdv1beta1.QuestDB
			snap *crdv1beta1.QuestDBSnapshot

			volSnap     = &volumesnapshotv1.VolumeSnapshot{}
			preSnapJob  = &batchv1.Job{}
			postSnapJob = &batchv1.Job{}
		)

		BeforeAll(func() {
			q = testutils.BuildAndCreateMockQuestDB(ctx, k8sClient)
			snap = testutils.BuildAndCreateMockQuestDBSnapshot(ctx, k8sClient, q)
		})

		It("Should create a pre-snapshot job when a QuestDBSnapshot is created", func() {
			By("Checking if a pre-snapshot job is created")
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{
					Name:      fmt.Sprintf("%s-pre-snapshot", snap.Name),
					Namespace: snap.Namespace,
				}, preSnapJob)
			}, timeout, interval).Should(Succeed())

			By("Checking if the phase is set to SnapshotPending")
			Eventually(func() crdv1beta1.QuestDBSnapshotPhase {
				k8sClient.Get(ctx, client.ObjectKey{
					Name:      snap.Name,
					Namespace: snap.Namespace,
				}, snap)
				return snap.Status.Phase
			}, timeout, interval).Should(Equal(crdv1beta1.SnapshotPending))

			By("Ensuring that the phase does not mutate if the pre-snapshot job is not complete")
			Consistently(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, client.ObjectKey{
					Name:      preSnapJob.Name,
					Namespace: preSnapJob.Namespace,
				}, preSnapJob)).Should(Succeed())
				g.Expect(preSnapJob.Status.Succeeded).Should(Equal(int32(0)))

				g.Expect(k8sClient.Get(ctx, client.ObjectKey{
					Name:      snap.Name,
					Namespace: snap.Namespace,
				}, snap)).Should(Succeed())
				g.Expect(snap.Status.Phase).Should(Equal(crdv1beta1.SnapshotPending))

			}, consistencyTimeout, interval).Should(Succeed())
		})

		It("Should create a VolumeSnapshot once the pre-snapshot job is complete", func() {
			By("Getting the pre-snapshot job")
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{
					Name:      preSnapJob.Name,
					Namespace: preSnapJob.Namespace,
				}, preSnapJob)
			}, timeout, interval).Should(Succeed())

			By("Manually completing the pre-snapshot job")
			// Since this is an ordered test, job should not be nil from the previous step
			Expect(preSnapJob).ShouldNot(BeNil())
			preSnapJob.Status.Succeeded = 1
			Expect(k8sClient.Status().Update(ctx, preSnapJob)).To(Succeed())

			By("Checking if the phase is set to SnapshotRunning")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, client.ObjectKey{
					Name: snap.Name,

					Namespace: snap.Namespace,
				}, snap)).Should(Succeed())

				g.Expect(snap.Status.Phase).Should(Equal(crdv1beta1.SnapshotRunning))
			}, timeout, interval).Should(Succeed())

			By("Checking if a VolumeSnapshot is created")
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{
					Name:      snap.Name,
					Namespace: snap.Namespace,
				}, volSnap)
			}, timeout, interval).Should(Succeed())

			By("Ensuring that the phase does not mutate if the VolumeSnapshot is not ready")
			Consistently(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, client.ObjectKey{
					Name:      snap.Name,
					Namespace: snap.Namespace,
				}, snap)).Should(Succeed())
				g.Expect(snap.Status.Phase).Should(Equal(crdv1beta1.SnapshotRunning))

				g.Expect(k8sClient.Get(ctx, client.ObjectKey{
					Name:      volSnap.Name,
					Namespace: volSnap.Namespace,
				}, volSnap)).Should(Succeed())

				if volSnap.Status == nil {
					// Forcefully set the volume snapshot status to not nil so that the test does not fail
					volSnap.Status = &volumesnapshotv1.VolumeSnapshotStatus{
						ReadyToUse: pointer.Bool(false),
					}
					k8sClient.Status().Update(ctx, volSnap)
				} else {
					g.Expect(*volSnap.Status.ReadyToUse).Should(BeFalse())
				}
			}, consistencyTimeout, interval).Should(Succeed())
		})

		It("Should create a post-snapshot job once the VolumeSnapshot is ready", func() {
			By("Setting the ready to use condition to true")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, client.ObjectKey{
					Name:      volSnap.Name,
					Namespace: volSnap.Namespace,
				}, volSnap)).Should(Succeed())
				if volSnap.Status == nil {
					volSnap.Status = &volumesnapshotv1.VolumeSnapshotStatus{}
				}
				volSnap.Status.ReadyToUse = pointer.Bool(true)
				g.Expect(k8sClient.Status().Update(ctx, volSnap)).To(Succeed())
			}, timeout, interval).Should(Succeed())

			By("Waiting for the phase to be set to SnapshotFinalizing")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, client.ObjectKey{
					Name:      snap.Name,
					Namespace: snap.Namespace,
				}, snap)).Should(Succeed())

				g.Expect(snap.Status.Phase).Should(Equal(crdv1beta1.SnapshotFinalizing))
			}, timeout, interval).Should(Succeed())

			By("Checking if a post-snapshot job is created")
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{
					Name:      fmt.Sprintf("%s-post-snapshot", snap.Name),
					Namespace: snap.Namespace,
				}, postSnapJob)
			}, timeout, interval).Should(Succeed())

		})

		It("Should set the phase to SnapshotSucceeded once the post-snapshot job is complete", func() {
			By("Setting the post-snapshot job to complete")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, client.ObjectKey{
					Name:      postSnapJob.Name,
					Namespace: postSnapJob.Namespace,
				}, postSnapJob)).To(Succeed())
				postSnapJob.Status.Succeeded = 1
				g.Expect(k8sClient.Status().Update(ctx, postSnapJob)).To(Succeed())
			}, timeout, interval).Should(Succeed())

			By("Checking if the phase is set to SnapshotSucceeded")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, client.ObjectKey{
					Name:      snap.Name,
					Namespace: snap.Namespace,
				}, snap)).Should(Succeed())

				g.Expect(snap.Status.Phase).Should(Equal(crdv1beta1.SnapshotSucceeded))
			}, timeout, interval).Should(Succeed())
		})

		It("Should clean up the jobs once the snapshot has succeeded", func() {
			By("Checking that the pre-snapshot job is deleted")
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{
					Name:      preSnapJob.Name,
					Namespace: preSnapJob.Namespace,
				}, preSnapJob)
			}, timeout, interval).ShouldNot(Succeed())

			By("Checking that the post-snapshot job is deleted")
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{
					Name:      postSnapJob.Name,
					Namespace: postSnapJob.Namespace,
				}, postSnapJob)
			}, timeout, interval).ShouldNot(Succeed())

		})

	})

	Context("failure cases", func() {
		var (
			q    *crdv1beta1.QuestDB
			snap *crdv1beta1.QuestDBSnapshot
		)

		When("a pre snapshot job fails", Ordered, func() {
			var (
				job = &batchv1.Job{}
			)
			BeforeAll(func() {
				q = testutils.BuildAndCreateMockQuestDB(ctx, k8sClient)
				snap = testutils.BuildAndCreateMockQuestDBSnapshot(ctx, k8sClient, q)

				By("Waiting for the pre-snapshot job to be created")
				Eventually(func() error {
					return k8sClient.Get(ctx, client.ObjectKey{
						Name:      fmt.Sprintf("%s-pre-snapshot", snap.Name),
						Namespace: snap.Namespace,
					}, job)
				}, timeout, interval).Should(Succeed())
			})

			It("Should set the phase to SnapshotFailed", func() {
				By("Setting the failure condition on the pre-snapshot job")
				job.Status.Failed = snap.Spec.JobBackoffLimit
				Expect(k8sClient.Status().Update(ctx, job)).To(Succeed())

				By("Checking if the phase is set to SnapshotFailed")
				Eventually(func(g Gomega) {
					g.Expect(k8sClient.Get(ctx, client.ObjectKey{
						Name:      snap.Name,
						Namespace: snap.Namespace,
					}, snap)).Should(Succeed())

					g.Expect(snap.Status.Phase).Should(Equal(crdv1beta1.SnapshotFailed))
				}, timeout, interval).Should(Succeed())
			})

			It("Should not create the snapshot finalizer", func() {
				By("Setting the failure condition on the pre-snapshot job")
				job.Status.Failed = snap.Spec.JobBackoffLimit
				Expect(k8sClient.Status().Update(ctx, job)).To(Succeed())

				By("Ensuring that the finalizer is never created")
				Consistently(func(g Gomega) {
					g.Expect(k8sClient.Get(ctx, client.ObjectKey{
						Name:      snap.Name,
						Namespace: snap.Namespace,
					}, snap)).Should(Succeed())

					g.Expect(snap.Finalizers).Should(HaveLen(0))
				})
			})

		})

		When("a post snapshot job fails", Ordered, func() {
			var (
				job = &batchv1.Job{}
			)

			BeforeAll(func() {
				q = testutils.BuildAndCreateMockQuestDB(ctx, k8sClient)
				snap = testutils.BuildAndCreateMockQuestDBSnapshot(ctx, k8sClient, q)
			})

			It("Should set the phase to SnapshotFailed", func() {
				By("Setting the phase to SnapshotFinalizing")
				Eventually(func(g Gomega) {
					g.Expect(k8sClient.Get(ctx, client.ObjectKey{
						Name:      snap.Name,
						Namespace: snap.Namespace,
					}, snap)).Should(Succeed())
					snap.Status.Phase = crdv1beta1.SnapshotFinalizing
					g.Expect(k8sClient.Status().Update(ctx, snap)).To(Succeed())
				}, timeout, interval).Should(Succeed())

				By("Waiting for the post-snapshot job to be created")
				Eventually(func() error {
					return k8sClient.Get(ctx, client.ObjectKey{
						Name:      fmt.Sprintf("%s-post-snapshot", snap.Name),
						Namespace: snap.Namespace,
					}, job)
				}, timeout, interval).Should(Succeed())

				By("Setting the failure condition on the post-snapshot job")
				job.Status.Failed = snap.Spec.JobBackoffLimit
				Expect(k8sClient.Status().Update(ctx, job)).To(Succeed())

				By("Checking if the phase is set to SnapshotFailed")
				Eventually(func(g Gomega) {
					g.Expect(k8sClient.Get(ctx, client.ObjectKey{
						Name:      snap.Name,
						Namespace: snap.Namespace,
					}, snap)).Should(Succeed())

					g.Expect(snap.Status.Phase).Should(Equal(crdv1beta1.SnapshotFailed))
				}, timeout, interval).Should(Succeed())
			})
		})

	})

	Context("finalizer tests", func() {
		var (
			snap *crdv1beta1.QuestDBSnapshot
		)

		BeforeEach(func() {
			snap = testutils.BuildAndCreateMockQuestDBSnapshot(ctx, k8sClient, testutils.BuildAndCreateMockQuestDB(ctx, k8sClient))

			By("Adding the finalizer")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: snap.Name, Namespace: snap.Namespace}, snap)).To(Succeed())
				snap.Finalizers = append(snap.Finalizers, crdv1beta1.SnapshotCompleteFinalizer)
				g.Expect(k8sClient.Update(ctx, snap)).Should(Succeed())
			}, timeout, interval).Should(Succeed())

		})

		It("Should delete the snapshot finalizer if the snapshot has failed", func() {
			By("Setting the phase to SnapshotFailed")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: snap.Name, Namespace: snap.Namespace}, snap)).To(Succeed())
				snap.Status.Phase = crdv1beta1.SnapshotFailed
				g.Expect(k8sClient.Status().Update(ctx, snap)).To(Succeed())
			}, timeout, interval).Should(Succeed())

			By("Checking if the snapshot finalizer is removed")
			Eventually(func(g Gomega) {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: snap.Name, Namespace: snap.Namespace}, snap)
				if !apierrors.IsNotFound(err) {
					g.Expect(err).To(Succeed())
					g.Expect(snap.Finalizers).NotTo(ContainElement(crdv1beta1.SnapshotCompleteFinalizer))
				}
			}, timeout, interval).Should(Succeed())
		})

		It("Should delete the snapshot finalizer if the snapshot has succeeded", func() {
			By("Setting the phase to SnapshotSucceeded")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: snap.Name, Namespace: snap.Namespace}, snap)).To(Succeed())
				snap.Status.Phase = crdv1beta1.SnapshotSucceeded
				g.Expect(k8sClient.Status().Update(ctx, snap)).To(Succeed())
			}, timeout, interval).Should(Succeed())

			By("Checking if the snapshot finalizer is removed")
			Eventually(func(g Gomega) {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: snap.Name, Namespace: snap.Namespace}, snap)
				if !apierrors.IsNotFound(err) {
					g.Expect(err).To(Succeed())
					g.Expect(snap.Finalizers).NotTo(ContainElement(crdv1beta1.SnapshotCompleteFinalizer))
				}
			}, timeout, interval).Should(Succeed())
		})

		It("Should not delete the snapshot finalizer if the snapshot is running", func() {
			By("Setting the phase to SnapshotRunning")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: snap.Name, Namespace: snap.Namespace}, snap)).To(Succeed())
				snap.Status.Phase = crdv1beta1.SnapshotRunning
				g.Expect(k8sClient.Status().Update(ctx, snap)).To(Succeed())
			}, timeout, interval).Should(Succeed())

			By("Deleting the QuestDBSnapshot")
			Expect(k8sClient.Delete(ctx, snap)).To(Succeed())

			By("Checking if the snapshot finalizer is still present")
			Consistently(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: snap.Name, Namespace: snap.Namespace}, snap)).To(Succeed())
				g.Expect(snap.Finalizers).To(ContainElement(crdv1beta1.SnapshotCompleteFinalizer))
			}, consistencyTimeout, interval).Should(Succeed())
		})

		Context("If a snapshot is finalizing", func() {
			var (
				job = &batchv1.Job{}
			)
			BeforeEach(func() {
				Eventually(func(g Gomega) {
					By("Setting the phase to SnapshotFinalizing")
					g.Expect(k8sClient.Get(
						ctx,
						client.ObjectKey{Name: snap.Name, Namespace: snap.Namespace},
						snap)).To(Succeed())
					snap.Status.Phase = crdv1beta1.SnapshotFinalizing
					g.Expect(k8sClient.Status().Update(ctx, snap)).To(Succeed())
				}, timeout, interval).Should(Succeed())

				By("Waiting for the post-snapshot job to be created")
				Eventually(func() error {
					return k8sClient.Get(ctx, client.ObjectKey{
						Name:      fmt.Sprintf("%s-post-snapshot", snap.Name),
						Namespace: snap.Namespace,
					}, job)
				}, timeout, interval).Should(Succeed())

				By("Deleting the QuestDBSnapshot")
				Expect(k8sClient.Delete(ctx, snap)).To(Succeed())

				By("Checking if the snapshot finalizer is still present")
				Consistently(func(g Gomega) {
					g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: snap.Name, Namespace: snap.Namespace}, snap)).To(Succeed())
					g.Expect(snap.Finalizers).To(ContainElement(crdv1beta1.SnapshotCompleteFinalizer))
				}, consistencyTimeout, interval).Should(Succeed())
			})

			It("Should not delete the snapshot finalizer if the post-snapshot job has failed, but fewer than the max failure count", func() {
				By("Incrementing the post-snapshot job failure count")
				Eventually(func(g Gomega) {
					g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: job.Name, Namespace: job.Namespace}, job)).To(Succeed())
					job.Status.Failed = snap.Spec.JobBackoffLimit - 1
					g.Expect(k8sClient.Status().Update(ctx, job)).To(Succeed())
				}, timeout, interval).Should(Succeed())

				By("Checking if the snapshot finalizer is still present")
				Consistently(func(g Gomega) {
					g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: snap.Name, Namespace: snap.Namespace}, snap)).To(Succeed())
					g.Expect(snap.Finalizers).To(ContainElement(crdv1beta1.SnapshotCompleteFinalizer))
				}, consistencyTimeout, interval).Should(Succeed())

			})

			It("Should not delete the snapshot finalizer if there is an active job", func() {

				By("Incrementing the post-snapshot job active count")
				Eventually(func(g Gomega) {
					g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: job.Name, Namespace: job.Namespace}, job)).To(Succeed())
					job.Status.Active = 1
					job.Status.Failed = snap.Spec.JobBackoffLimit - 1
					g.Expect(k8sClient.Status().Update(ctx, job)).To(Succeed())
				}, timeout, interval).Should(Succeed())

				By("Checking if the snapshot finalizer is still present")
				Consistently(func(g Gomega) {
					g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: snap.Name, Namespace: snap.Namespace}, snap)).To(Succeed())
					g.Expect(snap.Finalizers).To(ContainElement(crdv1beta1.SnapshotCompleteFinalizer))
				}, consistencyTimeout, interval).Should(Succeed())

			})

			It("Should delete the snapshot finalizer if the post-snapshot job is complete", func() {

				By("Setting the post-snapshot job to complete")
				Eventually(func(g Gomega) {
					g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: job.Name, Namespace: job.Namespace}, job)).To(Succeed())
					job.Status.Succeeded = 1
					g.Expect(k8sClient.Status().Update(ctx, job)).To(Succeed())
				}, timeout, interval).Should(Succeed())

				By("Checking if the snapshot finalizer is removed")
				Eventually(func(g Gomega) {
					err := k8sClient.Get(ctx, client.ObjectKey{Name: snap.Name, Namespace: snap.Namespace}, snap)
					if !apierrors.IsNotFound(err) {
						g.Expect(err).To(Succeed())
						g.Expect(snap.Finalizers).NotTo(ContainElement(crdv1beta1.SnapshotCompleteFinalizer))
					}
				}, timeout, interval).Should(Succeed())
			})

			It("Should delete the snapshot finalizer if the post-snapshot job has failed", func() {

				By("Setting the post-snapshot job to complete")
				Eventually(func(g Gomega) {
					g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: job.Name, Namespace: job.Namespace}, job)).To(Succeed())
					job.Status.Failed = snap.Spec.JobBackoffLimit
					g.Expect(k8sClient.Status().Update(ctx, job)).To(Succeed())
				}, timeout, interval).Should(Succeed())

				By("Checking if the snapshot finalizer is removed")
				Eventually(func(g Gomega) {
					err := k8sClient.Get(ctx, client.ObjectKey{Name: snap.Name, Namespace: snap.Namespace}, snap)
					if !apierrors.IsNotFound(err) {
						g.Expect(err).To(Succeed())
						g.Expect(snap.Finalizers).NotTo(ContainElement(crdv1beta1.SnapshotCompleteFinalizer))
					}
				}, timeout, interval).Should(Succeed())
			})

		})

		Context("If a snapshot is pending", func() {
			var (
				job = &batchv1.Job{}
			)
			BeforeEach(func() {
				Eventually(func(g Gomega) {
					By("Setting the phase to SnapshotPending")
					g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: snap.Name, Namespace: snap.Namespace}, snap)).To(Succeed())
					snap.Status.Phase = crdv1beta1.SnapshotPending
					g.Expect(k8sClient.Status().Update(ctx, snap)).To(Succeed())
				}, timeout, interval).Should(Succeed())

				By("Waiting for the pre-snapshot job to be created")
				Eventually(func() error {
					return k8sClient.Get(ctx, client.ObjectKey{
						Name:      fmt.Sprintf("%s-pre-snapshot", snap.Name),
						Namespace: snap.Namespace,
					}, job)
				}, timeout, interval).Should(Succeed())

				By("Deleting the QuestDBSnapshot")
				Expect(k8sClient.Delete(ctx, snap)).To(Succeed())

				By("Checking if the snapshot finalizer is still present")
				Consistently(func(g Gomega) {
					g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: snap.Name, Namespace: snap.Namespace}, snap)).To(Succeed())
					g.Expect(snap.Finalizers).To(ContainElement(crdv1beta1.SnapshotCompleteFinalizer))
				}, consistencyTimeout, interval).Should(Succeed())

			})

			It("Should not delete the snapshot finalizer if the number of failed jobs is less than the specified backoff limit", func() {
				By("Incrementing the pre-snapshot job failure count")
				Eventually(func(g Gomega) {
					g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: job.Name, Namespace: job.Namespace}, job)).To(Succeed())
					job.Status.Failed = snap.Spec.JobBackoffLimit - 1
					g.Expect(k8sClient.Status().Update(ctx, job)).To(Succeed())
				}, timeout, interval).Should(Succeed())

				By("Checking if the snapshot finalizer is still present")
				Consistently(func(g Gomega) {
					g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: snap.Name, Namespace: snap.Namespace}, snap)).To(Succeed())
					g.Expect(snap.Finalizers).To(ContainElement(crdv1beta1.SnapshotCompleteFinalizer))
				}, consistencyTimeout, interval).Should(Succeed())
			})

			It("Should not delete the snapshot finalizer if there is an active job", func() {
				By("Incrementing the pre-snapshot job active count")
				Eventually(func(g Gomega) {
					g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: job.Name, Namespace: job.Namespace}, job)).To(Succeed())
					job.Status.Active = 1
					job.Status.Failed = snap.Spec.JobBackoffLimit - 1
					g.Expect(k8sClient.Status().Update(ctx, job)).To(Succeed())
				}, timeout, interval).Should(Succeed())

				By("Checking if the snapshot finalizer is still present")
				Consistently(func(g Gomega) {
					g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: snap.Name, Namespace: snap.Namespace}, snap)).To(Succeed())
					g.Expect(snap.Finalizers).To(ContainElement(crdv1beta1.SnapshotCompleteFinalizer))
				}, consistencyTimeout, interval).Should(Succeed())
			})

			It("Should not delete the snapshot until the post-snapshot job has failed", func() {

				By("Setting the pre-snapshot failure count to the backoff limit")
				Eventually(func(g Gomega) {
					g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: job.Name, Namespace: job.Namespace}, job)).To(Succeed())
					job.Status.Failed = snap.Spec.JobBackoffLimit
					g.Expect(k8sClient.Status().Update(ctx, job)).To(Succeed())
				}, timeout, interval).Should(Succeed())

				By("Checking if snapshot is deleted")
				Eventually(func(g Gomega) {
					err := k8sClient.Get(ctx, client.ObjectKey{Name: snap.Name, Namespace: snap.Namespace}, snap)
					g.Expect(apierrors.IsNotFound(err))
				}, timeout, interval).Should(Succeed())
			})

			It("Should move the snapshot to finalizing if the post-snapshot job has succeeded", func() {
				By("Setting the pre-snapshot job success count = 1")
				Eventually(func(g Gomega) {
					g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: job.Name, Namespace: job.Namespace}, job)).To(Succeed())
					job.Status.Succeeded = 1
					g.Expect(k8sClient.Status().Update(ctx, job)).To(Succeed())
				}, timeout, interval).Should(Succeed())

				By("Checking if snapshot is deleted")
				Eventually(func(g Gomega) {
					g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: snap.Name, Namespace: snap.Namespace}, snap)).To(Succeed())
					g.Expect(snap.Finalizers).To(ContainElement(crdv1beta1.SnapshotCompleteFinalizer))
					g.Expect(snap.Status.Phase).To(Equal(crdv1beta1.SnapshotFinalizing))
				}, timeout, interval).Should(Succeed())
			})

		})
	})

	It("Should set the value of backoff limit to the default if it is not set", func() {
		snap := testutils.BuildAndCreateMockQuestDBSnapshot(ctx, k8sClient, testutils.BuildAndCreateMockQuestDB(ctx, k8sClient))
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: snap.Name, Namespace: snap.Namespace}, snap)).To(Succeed())
			snap.Spec.JobBackoffLimit = 0
			g.Expect(k8sClient.Update(ctx, snap)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		By("Checking if the backoff limit is set to the default value")
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: snap.Name, Namespace: snap.Namespace}, snap)).To(Succeed())
			g.Expect(snap.Spec.JobBackoffLimit).To(Equal(crdv1beta1.JobBackoffLimitDefault))
		}, timeout, interval).Should(Succeed())
	})

	Context("Validation tests", func() {
		It("Should fail the snapshot immediately if the QuestDB does not exist", func() {
			// QuestDB is not actually created, but namespace is
			q := testutils.BuildMockQuestDB(ctx, k8sClient)
			snap := testutils.BuildAndCreateMockQuestDBSnapshot(ctx, k8sClient, q)
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: snap.Name, Namespace: snap.Namespace}, snap)).To(Succeed())
				g.Expect(snap.Status.Phase).To(Equal(crdv1beta1.SnapshotFailed))
			}, consistencyTimeout, interval).Should(Succeed())
		})

		It("Should fail the snapshot in the running phase if the VolumeSnapshot class does not exist", func() {
			q := testutils.BuildAndCreateMockQuestDB(ctx, k8sClient)
			// QuestDBSnapshot is not actually created so we can update the VolumeSnapshotClassName to something that does not exist
			snap := testutils.BuildMockQuestDBSnapshot(ctx, k8sClient, q)
			snap.Spec.VolumeSnapshotClassName = pointer.String("non-existent-class")
			// Now create the QuestDBSnapshot
			Expect(k8sClient.Create(ctx, snap)).To(Succeed())

			By("Checking if the phase is set to SnapshotFailed")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, client.ObjectKey{
					Name:      snap.Name,
					Namespace: snap.Namespace,
				}, snap)).Should(Succeed())

				g.Expect(snap.Status.Phase).Should(Equal(crdv1beta1.SnapshotFailed))
			}, timeout, interval).Should(Succeed())
		})
	})

})
