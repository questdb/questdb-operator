/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"
	"time"

	volumesnapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v8/apis/volumesnapshot/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	crdv1beta2 "github.com/questdb/questdb-operator/api/v1beta2"
)

var _ = Describe("QuestDBBackup Controller", func() {
	const (
		qdbName = "bkp-qdb"
		ns      = "default"
	)
	ctx := context.Background()
	var (
		fake       *fakeCheckpointer
		reconciler *QuestDBBackupReconciler
	)

	reconcileBackup := func(name string) error {
		_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: name, Namespace: ns}})
		return err
	}
	getBackup := func(name string) *crdv1beta2.QuestDBBackup {
		b := &crdv1beta2.QuestDBBackup{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: ns}, b)).To(Succeed())
		return b
	}
	newBackup := func(name string) *crdv1beta2.QuestDBBackup {
		b := &crdv1beta2.QuestDBBackup{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
			Spec:       crdv1beta2.QuestDBBackupSpec{QuestDBName: qdbName, Method: crdv1beta2.BackupMethodVolumeSnapshot},
		}
		Expect(k8sClient.Create(ctx, b)).To(Succeed())
		return b
	}
	markSnapshotReady := func(name string) {
		vs := &volumesnapshotv1.VolumeSnapshot{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: ns}, vs)).To(Succeed())
		vs.Status = &volumesnapshotv1.VolumeSnapshotStatus{ReadyToUse: ptr.To(true)}
		Expect(k8sClient.Status().Update(ctx, vs)).To(Succeed())
	}

	BeforeEach(func() {
		qdb := &crdv1beta2.QuestDB{
			ObjectMeta: metav1.ObjectMeta{Name: qdbName, Namespace: ns},
			Spec:       crdv1beta2.QuestDBSpec{Volume: crdv1beta2.QuestDBVolumeSpec{Size: resource.MustParse("1Gi")}},
		}
		Expect(client.IgnoreAlreadyExists(k8sClient.Create(ctx, qdb))).To(Succeed())

		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: qdbName + "-credentials", Namespace: ns},
			Data:       map[string][]byte{envPgUser: []byte("admin"), envPgPassword: []byte("secret")},
		}
		Expect(client.IgnoreAlreadyExists(k8sClient.Create(ctx, secret))).To(Succeed())

		// The QuestDB controller creates the data PVC; the backup controller pre-flights its existence
		// before opening a checkpoint, so the test must provide it.
		pvc := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{Name: qdbName, Namespace: ns},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Gi")},
				},
			},
		}
		Expect(client.IgnoreAlreadyExists(k8sClient.Create(ctx, pvc))).To(Succeed())

		fake = &fakeCheckpointer{}
		reconciler = &QuestDBBackupReconciler{
			Client:       k8sClient,
			Scheme:       k8sClient.Scheme(),
			Recorder:     record.NewFakeRecorder(100),
			Checkpointer: fake,
		}
	})

	AfterEach(func() {
		var list crdv1beta2.QuestDBBackupList
		Expect(k8sClient.List(ctx, &list, client.InNamespace(ns))).To(Succeed())
		for i := range list.Items {
			b := &list.Items[i]
			if controllerutil.RemoveFinalizer(b, crdv1beta2.BackupFinalizer) {
				_ = k8sClient.Update(ctx, b)
			}
			_ = k8sClient.Delete(ctx, b)
		}
		_ = k8sClient.DeleteAllOf(ctx, &volumesnapshotv1.VolumeSnapshot{}, client.InNamespace(ns))
	})

	It("runs the full lifecycle and releases the checkpoint on success", func() {
		newBackup("b1")

		Expect(reconcileBackup("b1")).To(Succeed()) // add finalizer
		Expect(controllerutil.ContainsFinalizer(getBackup("b1"), crdv1beta2.BackupFinalizer)).To(BeTrue())

		Expect(reconcileBackup("b1")).To(Succeed()) // CHECKPOINT CREATE
		b := getBackup("b1")
		Expect(b.Status.Phase).To(Equal(crdv1beta2.BackupPhaseCheckpointCreated))
		Expect(b.Status.CheckpointCreatedAt).NotTo(BeNil())
		Expect(fake.creates()).To(Equal(1))

		Expect(reconcileBackup("b1")).To(Succeed()) // create VolumeSnapshot, not ready yet
		vs := &volumesnapshotv1.VolumeSnapshot{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "b1", Namespace: ns}, vs)).To(Succeed())
		Expect(*vs.Spec.Source.PersistentVolumeClaimName).To(Equal(qdbName))

		markSnapshotReady("b1")
		Expect(reconcileBackup("b1")).To(Succeed()) // snapshot ready -> SnapshotCreated
		Expect(getBackup("b1").Status.Phase).To(Equal(crdv1beta2.BackupPhaseSnapshotCreated))

		Expect(reconcileBackup("b1")).To(Succeed()) // CHECKPOINT RELEASE -> Succeeded
		b = getBackup("b1")
		Expect(b.Status.Phase).To(Equal(crdv1beta2.BackupPhaseSucceeded))
		Expect(b.Status.CheckpointReleasedAt).NotTo(BeNil())
		Expect(fake.releases()).To(Equal(1))
	})

	It("releases the checkpoint when a backup is deleted mid-flight", func() {
		newBackup("b2")
		Expect(reconcileBackup("b2")).To(Succeed()) // finalizer
		Expect(reconcileBackup("b2")).To(Succeed()) // checkpoint created
		Expect(getBackup("b2").Status.Phase).To(Equal(crdv1beta2.BackupPhaseCheckpointCreated))

		Expect(k8sClient.Delete(ctx, getBackup("b2"))).To(Succeed())
		Expect(reconcileBackup("b2")).To(Succeed()) // delete path must release

		Expect(fake.releases()).To(BeNumerically(">=", 1))
		Eventually(func() bool {
			err := k8sClient.Get(ctx, types.NamespacedName{Name: "b2", Namespace: ns}, &crdv1beta2.QuestDBBackup{})
			return err != nil
		}, time.Second*5, time.Millisecond*100).Should(BeTrue())
	})

	It("releases the checkpoint when the snapshot exceeds the hold deadline", func() {
		newBackup("b3")
		Expect(reconcileBackup("b3")).To(Succeed()) // finalizer
		Expect(reconcileBackup("b3")).To(Succeed()) // checkpoint created

		// Backdate the checkpoint past the hold deadline so the next reconcile aborts.
		b := getBackup("b3")
		old := metav1.NewTime(time.Now().Add(-2 * checkpointHoldDeadline))
		b.Status.CheckpointCreatedAt = &old
		Expect(k8sClient.Status().Update(ctx, b)).To(Succeed())

		Expect(reconcileBackup("b3")).To(Succeed()) // create VS (not ready) + detect timeout -> abort + release
		b = getBackup("b3")
		Expect(b.Status.Phase).To(Equal(crdv1beta2.BackupPhaseFailed))
		Expect(b.Status.CheckpointReleasedAt).NotTo(BeNil())
		Expect(fake.releases()).To(BeNumerically(">=", 1))
	})

	It("aborts (and stops blocking) a backup whose CHECKPOINT CREATE keeps failing past the deadline", func() {
		fake.createErr = fmt.Errorf("connection refused")
		newBackup("b4")
		Expect(reconcileBackup("b4")).To(Succeed()) // finalizer
		Expect(reconcileBackup("b4")).To(Succeed()) // record obligation + Create fails -> requeue

		// The release obligation is recorded even though CHECKPOINT CREATE failed, and the backup has
		// not advanced past Pending (phase is still the empty/Pending state).
		b := getBackup("b4")
		Expect(b.Status.Phase).NotTo(Equal(crdv1beta2.BackupPhaseCheckpointCreated))
		Expect(b.Status.CheckpointCreatedAt).NotTo(BeNil())

		// Past the deadline the backup must reach a terminal state so it no longer blocks siblings.
		old := metav1.NewTime(time.Now().Add(-2 * checkpointHoldDeadline))
		b.Status.CheckpointCreatedAt = &old
		Expect(k8sClient.Status().Update(ctx, b)).To(Succeed())

		Expect(reconcileBackup("b4")).To(Succeed())
		Expect(getBackup("b4").Status.Phase).To(Equal(crdv1beta2.BackupPhaseFailed))
	})

	It("does not mark Succeeded while CHECKPOINT RELEASE is failing", func() {
		newBackup("b5")
		Expect(reconcileBackup("b5")).To(Succeed()) // finalizer
		Expect(reconcileBackup("b5")).To(Succeed()) // checkpoint created
		Expect(reconcileBackup("b5")).To(Succeed()) // create VolumeSnapshot
		markSnapshotReady("b5")
		Expect(reconcileBackup("b5")).To(Succeed()) // -> SnapshotCreated
		Expect(getBackup("b5").Status.Phase).To(Equal(crdv1beta2.BackupPhaseSnapshotCreated))

		fake.releaseErr = fmt.Errorf("connection refused")
		Expect(reconcileBackup("b5")).To(Succeed()) // release fails -> stay SnapshotCreated, not Succeeded
		b := getBackup("b5")
		Expect(b.Status.Phase).To(Equal(crdv1beta2.BackupPhaseSnapshotCreated))
		Expect(b.Status.CheckpointReleasedAt).To(BeNil())

		fake.releaseErr = nil
		Expect(reconcileBackup("b5")).To(Succeed()) // release succeeds -> Succeeded
		b = getBackup("b5")
		Expect(b.Status.Phase).To(Equal(crdv1beta2.BackupPhaseSucceeded))
		Expect(b.Status.CheckpointReleasedAt).NotTo(BeNil())
	})
})
