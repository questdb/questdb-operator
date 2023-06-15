package utils

import (
	"context"
	"fmt"
	"time"

	volumesnapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	. "github.com/onsi/ginkgo/v2" //lint:ignore ST1001 Ginkgo DSL
	. "github.com/onsi/gomega"    //lint:ignore ST1001 Gomega DSL

	crdv1beta1 "github.com/questdb/questdb-operator/api/v1beta1"
	v1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	StorageClassName  = "csi-hostpath-sc"
	CsiProvisioner    = "hostpath.csi.k8s.io"
	SnapshotClassName = "csi-hostpath-snapclass"
)

func BuildMockQuestDB(ctx context.Context, c client.Client) *crdv1beta1.QuestDB {
	var (
		name = "test-questdb"
		ns   = fmt.Sprintf("test-ns-%d", time.Now().UnixNano())
	)

	By("Creating a namespace")
	Expect(c.Create(ctx, &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: ns,
		},
	})).To(Succeed())

	return &crdv1beta1.QuestDB{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
			Labels: map[string]string{
				"app": "questdb",
			},
		},
		Spec: crdv1beta1.QuestDBSpec{
			Volume: crdv1beta1.QuestDBVolumeSpec{
				Size:             resource.MustParse("1Gi"),
				StorageClassName: pointer.String(StorageClassName),
			},
			Image: "questdb/questdb:latest",
		},
	}
}

func BuildAndCreateMockQuestDB(ctx context.Context, c client.Client) *crdv1beta1.QuestDB {

	By("Creating a QuestDB")
	q := BuildMockQuestDB(ctx, c)
	Expect(c.Create(ctx, q)).To(Succeed())
	return q
}

func BuildAndCreateMockQuestDBSnapshot(ctx context.Context, c client.Client, q *crdv1beta1.QuestDB) *crdv1beta1.QuestDBSnapshot {
	By("Creating a QuestDBSnapshot")
	snap := &crdv1beta1.QuestDBSnapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", q.Name, time.Now().Format("20060102150405")),
			Namespace: q.Namespace,
			Labels:    q.Labels,
		},
		Spec: crdv1beta1.QuestDBSnapshotSpec{
			QuestDBName:             q.Name,
			VolumeSnapshotClassName: SnapshotClassName,
			JobBackoffLimit:         5,
		},
	}

	Expect(c.Create(ctx, snap)).To(Succeed())

	return snap
}

func BuildAndCreateMockVolumeSnapshot(ctx context.Context, c client.Client, snap *crdv1beta1.QuestDBSnapshot) *volumesnapshotv1.VolumeSnapshot {
	By("Creating a VolumeSnapshot")
	volSnap := &volumesnapshotv1.VolumeSnapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", snap.Name, time.Now().Format("20060102150405")),
			Namespace: snap.Namespace,
			Labels:    snap.Labels,
		},
		Spec: volumesnapshotv1.VolumeSnapshotSpec{
			Source: volumesnapshotv1.VolumeSnapshotSource{
				PersistentVolumeClaimName: pointer.String(snap.Spec.QuestDBName),
			},
			VolumeSnapshotClassName: pointer.String(snap.Spec.VolumeSnapshotClassName),
		},
	}

	Expect(c.Create(ctx, volSnap)).To(Succeed())

	return volSnap

}

func BuildAndCreateMockQuestDBSnapshotSchedule(ctx context.Context, c client.Client, q *crdv1beta1.QuestDB) *crdv1beta1.QuestDBSnapshotSchedule {
	By("Creating a QuestDBSnapshotSchedule")
	sched := &crdv1beta1.QuestDBSnapshotSchedule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      q.Name,
			Namespace: q.Namespace,
			Labels:    q.Labels,
		},
		Spec: crdv1beta1.QuestDBSnapshotScheduleSpec{
			Snapshot: crdv1beta1.QuestDBSnapshotSpec{
				QuestDBName:             q.Name,
				VolumeSnapshotClassName: SnapshotClassName,
			},
			Schedule: "*/1 * * * *",
		},
	}

	Expect(c.Create(ctx, sched)).To(Succeed())

	return sched
}

func BuildAndCreateMockStorageClass(ctx context.Context, c client.Client) *storagev1.StorageClass {
	cls := &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: StorageClassName,
		},
		Parameters: map[string]string{
			"type": "hostpath",
		},
		Provisioner:          CsiProvisioner,
		AllowVolumeExpansion: pointer.Bool(true),
	}

	Expect(c.Create(ctx, cls)).To(Succeed())

	return cls
}
