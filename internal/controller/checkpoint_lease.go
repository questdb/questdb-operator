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
	"time"

	coordinationv1 "k8s.io/api/coordination/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	crdv1beta2 "github.com/questdb/questdb-operator/api/v1beta2"
)

// checkpointLeaseDuration bounds how long a checkpoint lease is honored without renewal. The holding
// backup renews it on every reconcile (well under this interval). It MUST exceed checkpointHoldDeadline:
// a crashed holder with an open checkpoint force-releases it via its own hold-deadline abort when the
// controller restarts; making the lease outlive that deadline guarantees no sibling can take the slot
// over (and open a second checkpoint on the same QuestDB) before the original holder has cleaned up.
const checkpointLeaseDuration = checkpointHoldDeadline + 5*time.Minute

// checkpointLeaseName is the Lease that serializes checkpoint access for a single QuestDB. QuestDB
// allows only one active checkpoint, so at most one backup per QuestDB may hold it at a time.
func checkpointLeaseName(questdbName string) string {
	return "questdb-checkpoint-" + questdbName
}

// acquireCheckpointLease atomically acquires (or renews) the checkpoint lease for a QuestDB on behalf
// of this backup, returning true only if this backup now holds it. Acquisition is serialized by the
// Lease object's optimistic concurrency (Create/Update conflict on resourceVersion), so concurrent
// reconciles can never both believe they hold the slot.
func (r *QuestDBBackupReconciler) acquireCheckpointLease(ctx context.Context, backup *crdv1beta2.QuestDBBackup, qdb *crdv1beta2.QuestDB) (bool, error) {
	name := checkpointLeaseName(backup.Spec.QuestDBName)
	holder := backup.Name
	now := metav1.NowMicro()
	durationSecs := ptr.To(int32(checkpointLeaseDuration.Seconds()))

	lease := &coordinationv1.Lease{}
	err := r.Get(ctx, types.NamespacedName{Namespace: backup.Namespace, Name: name}, lease)
	if apierrors.IsNotFound(err) {
		lease = &coordinationv1.Lease{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: backup.Namespace},
			Spec: coordinationv1.LeaseSpec{
				HolderIdentity:       &holder,
				LeaseDurationSeconds: durationSecs,
				AcquireTime:          &now,
				RenewTime:            &now,
			},
		}
		// Own the Lease by the QuestDB so it is garbage-collected when the QuestDB is deleted.
		if oerr := controllerutil.SetOwnerReference(qdb, lease, r.Scheme); oerr != nil {
			return false, oerr
		}
		if cerr := r.Create(ctx, lease); cerr != nil {
			// Lost the race to another backup that created it first; wait our turn.
			return false, client.IgnoreAlreadyExists(cerr)
		}
		return true, nil
	}
	if err != nil {
		return false, err
	}

	held := lease.Spec.HolderIdentity != nil && *lease.Spec.HolderIdentity == holder
	if !held && !leaseExpired(lease, now.Time) {
		// A different backup holds a live lease.
		return false, nil
	}
	if held && lease.Spec.RenewTime != nil && now.Sub(lease.Spec.RenewTime.Time) < checkpointLeaseDuration/2 {
		// Already ours and recently renewed; skip the write to avoid renewing on every reconcile.
		return true, nil
	}

	lease.Spec.HolderIdentity = &holder
	lease.Spec.LeaseDurationSeconds = durationSecs
	if !held {
		lease.Spec.AcquireTime = &now
	}
	lease.Spec.RenewTime = &now
	if uerr := r.Update(ctx, lease); uerr != nil {
		if apierrors.IsConflict(uerr) {
			// Another reconcile updated the lease first; retry on the next pass.
			return false, nil
		}
		return false, uerr
	}
	return true, nil
}

// leaseExpired reports whether a lease's renewal window has elapsed (so it may be taken over).
func leaseExpired(lease *coordinationv1.Lease, now time.Time) bool {
	if lease.Spec.RenewTime == nil || lease.Spec.LeaseDurationSeconds == nil {
		return true
	}
	expiry := lease.Spec.RenewTime.Add(time.Duration(*lease.Spec.LeaseDurationSeconds) * time.Second)
	return now.After(expiry)
}

// releaseCheckpointLeaseFor releases the checkpoint lease held by this backup (if any). It derives
// all identifiers from the backup so callers can't transpose namespace/questdbName/holder.
func (r *QuestDBBackupReconciler) releaseCheckpointLeaseFor(ctx context.Context, backup *crdv1beta2.QuestDBBackup) error {
	return r.releaseCheckpointLease(ctx, backup.Namespace, backup.Spec.QuestDBName, backup.Name)
}

// releaseCheckpointLease releases the checkpoint lease for a QuestDB, but only if this backup holds
// it (so a backup can't release a slot another backup has since acquired). Always call it after the
// checkpoint itself has been released.
func (r *QuestDBBackupReconciler) releaseCheckpointLease(ctx context.Context, namespace, questdbName, holder string) error {
	lease := &coordinationv1.Lease{}
	if err := r.Get(ctx, types.NamespacedName{Namespace: namespace, Name: checkpointLeaseName(questdbName)}, lease); err != nil {
		return client.IgnoreNotFound(err)
	}
	if lease.Spec.HolderIdentity == nil || *lease.Spec.HolderIdentity != holder {
		return nil
	}
	return client.IgnoreNotFound(r.Delete(ctx, lease))
}
