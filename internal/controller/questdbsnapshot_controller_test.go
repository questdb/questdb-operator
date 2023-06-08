package controller

import (
	"fmt"
	"time"

	volumesnapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	crdv1beta1 "github.com/questdb/questdb-operator/api/v1beta1"
)

var _ = Describe("QuestDBSnapshot Controller", func() {
	var (
		q *crdv1beta1.QuestDB

		timeout  = time.Second * 3
		interval = time.Millisecond * 100
	)

	BeforeEach(func() {
		var (
			name = "test-snapshot"
			ns   = fmt.Sprintf("test-ns-%d", time.Now().UnixNano())
		)

		By("Creating a namespace")
		Expect(k8sClient.Create(ctx, &v1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: ns,
			},
		})).To(Succeed())

		By("Creating a QuestDB")
		q = &crdv1beta1.QuestDB{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: ns,
				Labels: map[string]string{
					"app": "questdb",
				},
			},
			Spec: crdv1beta1.QuestDBSpec{
				Volume: crdv1beta1.QuestDBVolumeSpec{
					Size: resource.MustParse("1Gi"),
				},
				Image: "questdb/questdb:latest",
			},
		}

		Expect(k8sClient.Create(ctx, q)).To(Succeed())

	})

	Context("When a QuestDBSnapshot is created", Ordered, func() {
		var (
			snap    = &crdv1beta1.QuestDBSnapshot{}
			job     = &batchv1.Job{}
			volSnap = &volumesnapshotv1.VolumeSnapshot{}
		)

		BeforeAll(func() {
			By("Creating a QuestDBSnapshot")
			snap = &crdv1beta1.QuestDBSnapshot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("%s-%s", q.Name, time.Now().Format("20060102150405")),
					Namespace: q.Namespace,
					Labels:    q.Labels,
				},
				Spec: crdv1beta1.QuestDBSnapshotSpec{
					QuestDB:             q.Name,
					VolumeSnapshotClass: "csi-hostpath-snapclass",
				},
			}

			Expect(k8sClient.Create(ctx, snap)).To(Succeed())
		})

		It("Should create a pre-snapshot job when a QuestDBSnapshot is created", func() {
			By("Checking if a pre-snapshot job is created")
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{
					Name:      fmt.Sprintf("%s-pre-snapshot", snap.Name),
					Namespace: snap.Namespace,
				}, job)
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
					Name:      job.Name,
					Namespace: job.Namespace,
				}, job)).Should(Succeed())
				g.Expect(job.Status.Succeeded).Should(Equal(int32(0)))

				g.Expect(k8sClient.Get(ctx, client.ObjectKey{
					Name:      snap.Name,
					Namespace: snap.Namespace,
				}, snap)).Should(Succeed())
				g.Expect(snap.Status.Phase).Should(Equal(crdv1beta1.SnapshotPending))

			}, 1*time.Second, interval).Should(Succeed())
		})

		It("Should create a VolumeSnapshot once the pre-snapshot job is complete", func() {
			By("Manually completing the pre-snapshot job")
			// Since this is an ordered test, job should not be nil from the previous step
			Expect(job).ShouldNot(BeNil())
			job.Status.Succeeded = 1
			Expect(k8sClient.Status().Update(ctx, job)).To(Succeed())

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
			}, 1*time.Second, interval).Should(Succeed())
		})

		// todo: finish these tests

	})

})
