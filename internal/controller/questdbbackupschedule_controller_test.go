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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	crdv1beta2 "github.com/questdb/questdb-operator/api/v1beta2"
)

var _ = Describe("QuestDBBackupSchedule Controller", func() {
	const ns = "default"
	ctx := context.Background()
	var reconciler *QuestDBBackupScheduleReconciler

	reconcileSchedule := func(name string) error {
		_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: name, Namespace: ns}})
		return err
	}
	ownedBackups := func(sched *crdv1beta2.QuestDBBackupSchedule) []crdv1beta2.QuestDBBackup {
		var list crdv1beta2.QuestDBBackupList
		Expect(k8sClient.List(ctx, &list, client.InNamespace(ns))).To(Succeed())
		var owned []crdv1beta2.QuestDBBackup
		for i := range list.Items {
			if metav1.IsControlledBy(&list.Items[i], sched) && list.Items[i].DeletionTimestamp.IsZero() {
				owned = append(owned, list.Items[i])
			}
		}
		return owned
	}

	BeforeEach(func() {
		reconciler = &QuestDBBackupScheduleReconciler{
			Client:   k8sClient,
			Scheme:   k8sClient.Scheme(),
			Recorder: record.NewFakeRecorder(100),
			// Pretend "now" is an hour ahead so a freshly created schedule is already due.
			Now: func() time.Time { return time.Now().Add(time.Hour) },
		}
	})

	AfterEach(func() {
		_ = k8sClient.DeleteAllOf(ctx, &crdv1beta2.QuestDBBackupSchedule{}, client.InNamespace(ns))
		var list crdv1beta2.QuestDBBackupList
		Expect(k8sClient.List(ctx, &list, client.InNamespace(ns))).To(Succeed())
		for i := range list.Items {
			b := &list.Items[i]
			if controllerutil.RemoveFinalizer(b, crdv1beta2.BackupFinalizer) {
				_ = k8sClient.Update(ctx, b)
			}
			_ = k8sClient.Delete(ctx, b)
		}
	})

	It("creates a backup when the schedule is due", func() {
		sched := &crdv1beta2.QuestDBBackupSchedule{
			ObjectMeta: metav1.ObjectMeta{Name: "sched1", Namespace: ns},
			Spec: crdv1beta2.QuestDBBackupScheduleSpec{
				Schedule:  "* * * * *",
				Retention: 7,
				Backup:    crdv1beta2.QuestDBBackupSpec{QuestDBName: "some-qdb", Method: crdv1beta2.BackupMethodVolumeSnapshot},
			},
		}
		Expect(k8sClient.Create(ctx, sched)).To(Succeed())

		Expect(reconcileSchedule("sched1")).To(Succeed())

		owned := ownedBackups(sched)
		Expect(owned).To(HaveLen(1))
		Expect(owned[0].Spec.QuestDBName).To(Equal("some-qdb"))
	})

	It("prunes succeeded backups beyond the retention count", func() {
		sched := &crdv1beta2.QuestDBBackupSchedule{
			ObjectMeta: metav1.ObjectMeta{Name: "sched2", Namespace: ns},
			Spec: crdv1beta2.QuestDBBackupScheduleSpec{
				Schedule:  "* * * * *",
				Retention: 2,
				Suspend:   true, // don't create new backups; only test pruning
				Backup:    crdv1beta2.QuestDBBackupSpec{QuestDBName: "q", Method: crdv1beta2.BackupMethodVolumeSnapshot},
			},
		}
		Expect(k8sClient.Create(ctx, sched)).To(Succeed())

		for i := range 4 {
			b := &crdv1beta2.QuestDBBackup{
				ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("sched2-bk-%d", i), Namespace: ns},
				Spec:       crdv1beta2.QuestDBBackupSpec{QuestDBName: "q", Method: crdv1beta2.BackupMethodVolumeSnapshot},
			}
			Expect(controllerutil.SetControllerReference(sched, b, k8sClient.Scheme())).To(Succeed())
			Expect(k8sClient.Create(ctx, b)).To(Succeed())
			b.Status.Phase = crdv1beta2.BackupPhaseSucceeded
			Expect(k8sClient.Status().Update(ctx, b)).To(Succeed())
		}
		Expect(ownedBackups(sched)).To(HaveLen(4))

		Expect(reconcileSchedule("sched2")).To(Succeed())

		Eventually(func() int { return len(ownedBackups(sched)) }, time.Second*5, time.Millisecond*100).Should(Equal(2))
	})

	It("disables pruning when retention is negative", func() {
		sched := &crdv1beta2.QuestDBBackupSchedule{
			ObjectMeta: metav1.ObjectMeta{Name: "sched3", Namespace: ns},
			Spec: crdv1beta2.QuestDBBackupScheduleSpec{
				Schedule:  "* * * * *",
				Retention: -1, // disable pruning: keep all successful backups
				Suspend:   true,
				Backup:    crdv1beta2.QuestDBBackupSpec{QuestDBName: "q3", Method: crdv1beta2.BackupMethodVolumeSnapshot},
			},
		}
		Expect(k8sClient.Create(ctx, sched)).To(Succeed())

		for i := range 4 {
			b := &crdv1beta2.QuestDBBackup{
				ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("sched3-bk-%d", i), Namespace: ns},
				Spec:       crdv1beta2.QuestDBBackupSpec{QuestDBName: "q3", Method: crdv1beta2.BackupMethodVolumeSnapshot},
			}
			Expect(controllerutil.SetControllerReference(sched, b, k8sClient.Scheme())).To(Succeed())
			Expect(k8sClient.Create(ctx, b)).To(Succeed())
			b.Status.Phase = crdv1beta2.BackupPhaseSucceeded
			Expect(k8sClient.Status().Update(ctx, b)).To(Succeed())
		}
		Expect(ownedBackups(sched)).To(HaveLen(4))

		Expect(reconcileSchedule("sched3")).To(Succeed())

		// All four are retained; nothing is pruned.
		Consistently(func() int { return len(ownedBackups(sched)) }, time.Second*2, time.Millisecond*200).Should(Equal(4))
	})
})
