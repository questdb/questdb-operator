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

package v1beta2

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	crdv1beta2 "github.com/questdb/questdb-operator/api/v1beta2"
)

var _ = Describe("QuestDBBackupSchedule Webhook", func() {
	const ns = "default"

	newSchedule := func(name, schedule, questdbName string) *crdv1beta2.QuestDBBackupSchedule {
		return &crdv1beta2.QuestDBBackupSchedule{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
			Spec: crdv1beta2.QuestDBBackupScheduleSpec{
				Schedule: schedule,
				Backup:   crdv1beta2.QuestDBBackupSpec{QuestDBName: questdbName},
			},
		}
	}

	AfterEach(func() {
		_ = k8sClient.DeleteAllOf(ctx, &crdv1beta2.QuestDBBackupSchedule{}, client.InNamespace(ns))
	})

	It("defaults retention and backup method", func() {
		Expect(k8sClient.Create(ctx, newSchedule("whs-default", "0 2 * * *", "q"))).To(Succeed())

		f := &crdv1beta2.QuestDBBackupSchedule{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "whs-default", Namespace: ns}, f)).To(Succeed())
		Expect(f.Spec.Retention).To(Equal(int32(7)))
		Expect(f.Spec.Backup.Method).To(Equal(crdv1beta2.BackupMethodVolumeSnapshot))
	})

	It("rejects an invalid cron expression", func() {
		Expect(k8sClient.Create(ctx, newSchedule("whs-badcron", "not a cron", "q"))).NotTo(Succeed())
	})

	It("rejects an empty backup.questdbName", func() {
		Expect(k8sClient.Create(ctx, newSchedule("whs-noqdb", "0 2 * * *", ""))).NotTo(Succeed())
	})
})
