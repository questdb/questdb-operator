# Old snapshot flow (v1beta1 on `main`) — spec for the Phase 4 backup port

> **Status:** reference only. Captures the 2023 `QuestDBSnapshot` /
> `QuestDBSnapshotSchedule` behaviour as it exists on the `main` branch, verified by reading
> `git show main:<path>` and cross-checked by an adversarial review pass. This is the
> behavioural spec the Phase 4 `QuestDBBackup` / `QuestDBBackupSchedule` reconcilers must
> reproduce (with the SQL swapped to `CHECKPOINT CREATE/RELEASE` and the listed bugs fixed).
> Line numbers are approximate (drift of ±1 is expected); the file is the source of truth.

## Sources

- `internal/controller/questdbsnapshot_controller.go` (441 L) — the phase machine.
- `internal/controller/questdbsnapshotschedule_controller.go` (239 L) — cron + retention GC.
- `api/v1beta1/questdbsnapshot_types.go`, `questdbsnapshotschedule_types.go` — types/consts.
- `api/v1beta1/questdbsnapshot_webhook.go`, `questdbsnapshotschedule_webhook.go` — admission.
- `internal/secrets/` — credential discovery (annotation model; cross-ref Phase 3).

---

## 1. QuestDBSnapshot — types

```go
type QuestDBSnapshotSpec struct {
    QuestDBName             string  `json:"questdbName"`              // required
    VolumeSnapshotClassName *string `json:"volumeSnapshotClassName"` // optional; nil => cluster default
    JobBackoffLimit         int32   `json:"jobBackoffLimit"`          // webhook-defaulted to 5
}
type QuestDBSnapshotStatus struct {
    Phase QuestDBSnapshotPhase `json:"phase,omitempty"`              // the ONLY status field
}
```

- Phase enum (wire values): `Pending`, `Running`, `Finalizing`, `Failed`, `Succeeded`.
  The initial phase is the **empty string `""`** (zero value), handled by `handlePhaseEmpty`.
- Finalizer: `questdbsnapshot.crd.questdb.io/snapshot-complete-finalizer`.
- `JobBackoffLimitDefault = 5` (defined in the snapshot webhook, not `const.go`).
- Helper `IsComplete()` = `Phase == Succeeded || Phase == Failed`.
- Status carries **no** conditions, timestamps, or `volumeSnapshotName` — all of that is new
  in the v1beta2 `QuestDBBackup` design.

## 2. The phase machine

`Reconcile` gets the CR (`IgnoreNotFound`), defensively defaults `JobBackoffLimit` to 5 with a
spec `Update` if zero, then **dispatches on `status.phase` via a `switch`** (no top-level
`DeletionTimestamp` branch — see §4). Unknown phase → `fmt.Errorf("unknown phase %s")`.

`SetupWithManager`: `For(&QuestDBSnapshot{}).Owns(&VolumeSnapshot{}).Owns(&Job{})`. Progress is
**100% watch-driven** — there is no `RequeueAfter`/`Requeue` anywhere in the controller.

| From | To | Trigger / guard |
|---|---|---|
| `""` | `Failed` | parent QuestDB (`spec.questdbName`) not found (`IsNotFound`); Warning event |
| `""` | `Pending` | parent QuestDB exists *(also falls through on non-NotFound errors — latent bug)* |
| `Pending` | `Failed` | `spec.volumeSnapshotClassName` set but that `VolumeSnapshotClass` not found |
| `Pending` | `Running` | pre-snapshot Job `Succeeded==1` **and** `DeletionTimestamp == nil`; finalizer added first |
| `Pending` | `Finalizing` | pre-snapshot Job `Succeeded==1` **and** `DeletionTimestamp != nil` (delete shortcut) |
| `Pending` | `Failed` | pre-snapshot Job `Failed >= jobBackoffLimit` (finalizer was never added) |
| `Running` | `Finalizing` | `VolumeSnapshot.Status.ReadyToUse == true` |
| `Running` | `Running` | VolumeSnapshot not ready — **no failure/timeout path** (stall risk, see §4) |
| `Finalizing` | `Succeeded` | post-snapshot Job `Succeeded==1` |
| `Finalizing` | `Failed` | post-snapshot Job `Failed >= jobBackoffLimit` (**strands an open checkpoint**, see §4) |
| `Failed` | terminal | `handlePhaseFailed`: remove finalizer (no job cleanup) |
| `Succeeded` | terminal | `handlePhaseSucceeded`: remove finalizer, delete pre/post Jobs (`Background` propagation) |

Children (all owner-ref'd to the snapshot, so GC'd with it):
- **VolumeSnapshot** — `snapshot.storage.k8s.io/v1` (Go client `external-snapshotter/client/v6`).
  Name = `snap.Name`; `Source.PersistentVolumeClaimName = spec.questdbName` (snapshots the PVC
  named identically to the QuestDB); `VolumeSnapshotClassName` from spec (nil → default).
  Readiness gate: `Status != nil && Status.ReadyToUse != nil && *ReadyToUse`.
- **pre-snapshot Job** — name `<snap>-pre-snapshot`, runs `SNAPSHOT PREPARE;`.
- **post-snapshot Job** — name `<snap>-post-snapshot`, runs `SNAPSHOT COMPLETE;`.

## 3. How the SQL runs (CRITICAL mechanism)

**Not** pod-exec, **not** an in-process pg-wire driver, **not** HTTP `/exec`. Each SQL step is a
**`batch/v1` Job** (`buildGenericSnapshotJob`) whose single pod runs image **`postgres:13.3`**
(hardcoded, with a `// todo: make variable` comment) and invokes the `psql` CLI:

```
Command: ["psql", "-c", "<SQL>;"]        // "SNAPSHOT PREPARE;" or "SNAPSHOT COMPLETE;"
```

Connection via libpq env vars: `PGHOST=<spec.questdbName>.<ns>.svc.cluster.local`,
`PGPORT=8812` (pg-wire), `PGDATABASE=qdb`, `PGUSER`/`PGPASSWORD` default to **`admin`/`quest`**.
No TLS (`PGSSLMODE` unset). `Completions=1`, `RestartPolicy=Never`,
`BackoffLimit = jobBackoffLimit + 2` (the `+2` headroom lets the reconciler observe
`Failed >= jobBackoffLimit` before the Job itself stops retrying). The controller never opens a
DB connection — it only watches `Job.Status.Succeeded/Failed`.

**Credential key mismatch (real bug; matters for Phase 3):** the snapshot controller reads
`PsqlSecret.Data["QDB_PSQL_USER"]` / `["QDB_PSQL_PASSWORD"]`, but `internal/secrets` validates
and the QuestDB controller's `EnvFrom` use **`QDB_PG_USER` / `QDB_PG_PASSWORD`** (and tests seed
`QDB_PG_*`). The names never align, so the snapshot job's creds **always silently fall back to
`admin`/`quest`**. The secret is also looked up by `ObjectKeyFromObject(snap)` (the *snapshot's*
name) rather than `spec.questdbName`, so it usually resolves to nil anyway. Phase 3 must
standardize on one key naming (plan proposes `QDB_PG_*`) and resolve creds from the QuestDB.

## 4. Failure handling & the checkpoint invariant — what Phase 4 MUST fix

**Phase 4 invariant:** once `CHECKPOINT CREATE` (old: `SNAPSHOT PREPARE`) succeeds, `CHECKPOINT
RELEASE` (old: `SNAPSHOT COMPLETE`) must be attempted on **every** exit path — failure, requeue,
and deletion. The old code does **not** hold this. Verified violations:

1. **Stuck `Running`.** `handlePhaseRunning` returns `ctrl.Result{}, nil` with no failure,
   timeout, or requeue. If the VolumeSnapshot never reaches `ReadyToUse` (CSI crash, status
   never written), the machine sits in `Running` **forever** — COMPLETE never runs and the
   finalizer (added on PREPARE success) blocks CR deletion. DB left checkpoint-open.
2. **Deletion while `Running`.** `Reconcile` dispatches purely on `status.phase`; the
   `DeletionTimestamp → Finalizing` shortcut exists **only** in `handlePhasePending`. Deleting a
   CR in `Running` re-enters `handlePhaseRunning`, which advances only if the VolumeSnapshot is
   already ready — otherwise **deadlock** (finalizer set, deletion blocked, COMPLETE never runs).
3. **`Finalizing → Failed` abandons COMPLETE.** When the post-snapshot Job exhausts its backoff,
   phase → `Failed`; `handlePhaseFailed` then **only removes the finalizer** and returns — it
   never re-attempts COMPLETE. The CR becomes deletable with the DB still prepared. (The old
   code carries a `// todo: this is serious...` comment here.)

**Parts that are correct and must be preserved in Phase 4:**

- **Finalizer ordering.** The finalizer is added *inside* the PREPARE-`Succeeded==1` block and
  committed before/with the transition out of `Pending`. So "PREPARE succeeded" implies
  "finalizer present" before the obligation is recorded — a delete can't bypass the COMPLETE
  obligation at the moment of success. (Phase 4: set the RELEASE finalizer atomically with /
  before recording that CHECKPOINT CREATE succeeded.)
- **Delete-during-`Pending`-after-success** routes straight to `Finalizing` so COMPLETE runs.
- **PREPARE-failure needs no COMPLETE** — the finalizer was never added, so immediate deletion
  is correct (DB was never prepared).

**Required Phase 4 additions:** (i) a top-level deletion-aware path that always drives RELEASE
from any post-CREATE phase; (ii) a timeout/failure transition out of `Running` that still drives
RELEASE; (iii) a `Failed`-after-CREATE-success state that keeps retrying RELEASE (or holds the
finalizer) until RELEASE actually succeeds, instead of dropping it; (iv) an explicit
`RequeueAfter` while a RELEASE obligation is outstanding, so it retries without a watch event.

## 5. QuestDBSnapshotSchedule — cron + retention

```go
type QuestDBSnapshotScheduleSpec struct {
    Schedule  string              `json:"schedule"`   // 5-field cron (robfig/cron/v3, no seconds/@descriptors)
    Retention int32               `json:"retention"`  // keep-last-N succeeded; webhook default 7
    Snapshot  QuestDBSnapshotSpec `json:"snapshot"`   // template copied verbatim into each child
}
type QuestDBSnapshotScheduleStatus struct {
    LastSnapshot  metav1.Time          `json:"lastSnapshot,omitempty"`
    SnapshotPhase QuestDBSnapshotPhase `json:"snapshotPhase,omitempty"` // mirrors newest child
}
```

- **Cron:** `cron.NewParser(Minute|Hour|Dom|Month|Dow)`, rebuilt each reconcile (and duplicated
  in the webhook). Time via an injected `abtime.AbstractTime` (`ManualTime` in tests). Due when
  `crontab.Next(lastSnapshotTime).Compare(now) <= 0`. `getLastSnapshotTime` falls back to
  `CreationTimestamp` when `LastSnapshot` is zero. Requeue at `RequeueAfter: crontab.Next(now) - now`.
  Invalid cron → Warning `InvalidSchedule`, stop (no requeue) until edited.
- **Child creation** (`buildSnapshot`): name `<sched>-<YYYYMMDDHHMMSS>` (second precision),
  copies `sched.Labels` + `spec.snapshot`, owner-ref'd to the schedule (panics on ref error).
  Created only when due **and** (`status.snapshotPhase == ""` OR newest child `IsComplete()`),
  wrapped in `RetryOnConflict` + `IgnoreAlreadyExists`.
- **Retention GC** (`garbageCollect`): lists owned snapshots sorted newest-first, walks them, and
  deletes succeeded ones beyond the newest `retention`. **Only `Succeeded` snapshots count and
  are deleted — `Failed` snapshots accumulate unbounded.** Runs every reconcile.
- **Gotchas to carry forward or fix:** coarse overlap suppression (latest child across the whole
  list); **at-most-one catch-up** for missed ticks (no backfill / `StartingDeadlineSeconds`);
  `LastSnapshot` advances to `now` even on skip/create-failure (lost period); local-zone cron (no
  timezone field); second-precision name collisions; `O(all namespace snapshots)` in-memory
  filter (no indexer/selector).

## 6. Webhook rules to port (→ `internal/webhook/v1beta2/`)

- **QuestDBSnapshot** → **QuestDBBackup**: default `jobBackoffLimit=5`; create requires
  `questdbName != ""` and rejects `volumeSnapshotClassName` pointing to `""` (nil OK); update
  makes `questdbName`, `volumeSnapshotClassName`, `jobBackoffLimit` immutable; `ValidateDelete`
  is a no-op. (New: `spec.method` enum `VolumeSnapshot`.)
- **QuestDBSnapshotSchedule** → **QuestDBBackupSchedule**: default `snapshot.jobBackoffLimit=5`
  and `retention=7`; create/update parse the 5-field cron (reject invalid) and delegate to the
  backup-template create/update rules; `schedule` and `retention` are mutable.

## 7. v1beta2 / checkpoint translation summary (Phase 4)

| Old (v1beta1) | New (v1beta2) |
|---|---|
| `QuestDBSnapshot` | `QuestDBBackup` (+ `spec.method` enum: `VolumeSnapshot`) |
| `QuestDBSnapshotSchedule` | `QuestDBBackupSchedule` |
| `SNAPSHOT PREPARE;` | `CHECKPOINT CREATE;` — **HUMAN-VERIFY exact syntax on target version** |
| `SNAPSHOT COMPLETE;` | `CHECKPOINT RELEASE;` — **HUMAN-VERIFY**; handle single-active-checkpoint / "already exists" |
| phases `Pending/Running/Finalizing/Succeeded/Failed` | phases `Pending → CheckpointCreated → SnapshotCreated → CheckpointReleased → Complete \| Failed` (+ conditions); "checkpoint" appears in **status only** |
| status: `phase` only | status: phase enum, conditions, `volumeSnapshotName`, checkpoint create/release timestamps |
| creds `QDB_PSQL_*` (bug) / annotation discovery | `spec.auth` refs, `QDB_PG_*`, resolved from the QuestDB (Phase 3) |

**Open HUMAN-VERIFY items reached through this flow:** checkpoint SQL syntax/behaviour and the
single-active-checkpoint constraint (#3); the checkpoint-era restore trigger/config keys (#4);
default QuestDB image tag (#5). Restore intent lives in `config/samples/crd_v1beta1_questdb_from_snapshot.yaml`
on `main` (a QuestDB whose `spec.volume.snapshotName` sets a PVC `DataSource`).
