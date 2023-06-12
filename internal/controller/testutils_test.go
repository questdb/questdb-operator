package controller

import (
	"fmt"
	"time"

	volumesnapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	crdv1beta1 "github.com/questdb/questdb-operator/api/v1beta1"
	v1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

func buildMockQuestDB() *crdv1beta1.QuestDB {
	var (
		name = "test-questdb"
		ns   = fmt.Sprintf("test-ns-%d", time.Now().UnixNano())
	)

	By("Creating a namespace")
	Expect(k8sClient.Create(ctx, &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: ns,
		},
	})).To(Succeed())

	By("Creating a QuestDB")
	q := &crdv1beta1.QuestDB{
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
				StorageClassName: pointer.String("csi-hostpath-sc"),
			},
			Image: "questdb/questdb:latest",
		},
	}
	Expect(k8sClient.Create(ctx, q)).To(Succeed())

	return q
}

func buildMockQuestDBSnapshot(q *crdv1beta1.QuestDB) *crdv1beta1.QuestDBSnapshot {
	By("Creating a QuestDBSnapshot")
	snap := &crdv1beta1.QuestDBSnapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", q.Name, time.Now().Format("20060102150405")),
			Namespace: q.Namespace,
			Labels:    q.Labels,
		},
		Spec: crdv1beta1.QuestDBSnapshotSpec{
			QuestDBName:             q.Name,
			VolumeSnapshotClassName: "csi-hostpath-snapclass",
			JobBackoffLimit:         5,
		},
	}

	Expect(k8sClient.Create(ctx, snap)).To(Succeed())

	return snap
}

func buildMockVolumeSnapshot(snap *crdv1beta1.QuestDBSnapshot) *volumesnapshotv1.VolumeSnapshot {
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

	Expect(k8sClient.Create(ctx, volSnap)).To(Succeed())

	return volSnap

}

func buildMockStorageClass() *storagev1.StorageClass {
	cls := &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: "csi-hostpath-sc",
		},
		Parameters: map[string]string{
			"type": "hostpath",
		},
		Provisioner:          "hostpath.csi.k8s.io",
		AllowVolumeExpansion: pointer.Bool(true),
	}

	Expect(k8sClient.Create(ctx, cls)).To(Succeed())

	return cls
}
