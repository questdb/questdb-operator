# API Reference

## Packages
- [crd.questdb.io/v1beta1](#crdquestdbiov1beta1)


## crd.questdb.io/v1beta1

Package v1beta1 contains API Schema definitions for the crd v1beta1 API group

### Resource Types
- [QuestDB](#questdb)
- [QuestDBSnapshot](#questdbsnapshot)
- [QuestDBSnapshotSchedule](#questdbsnapshotschedule)



#### QuestDB



QuestDB is the Schema for the questdbs API



| Field | Description |
| --- | --- |
| `apiVersion` _string_ | `crd.questdb.io/v1beta1`
| `kind` _string_ | `QuestDB`
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |
| `spec` _[QuestDBSpec](#questdbspec)_ |  |
| `status` _[QuestDBStatus](#questdbstatus)_ |  |


#### QuestDBConfigSpec





_Appears in:_
- [QuestDBSpec](#questdbspec)

| Field | Description |
| --- | --- |
| `serverConfig` _string_ |  |
| `logConfig` _string_ |  |




#### QuestDBSnapshot



QuestDBSnapshot is the Schema for the snapshots API



| Field | Description |
| --- | --- |
| `apiVersion` _string_ | `crd.questdb.io/v1beta1`
| `kind` _string_ | `QuestDBSnapshot`
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |
| `spec` _[QuestDBSnapshotSpec](#questdbsnapshotspec)_ |  |
| `status` _[QuestDBSnapshotStatus](#questdbsnapshotstatus)_ |  |


#### QuestDBSnapshotSchedule



QuestDBSnapshotSchedule is the Schema for the snapshotschedules API



| Field | Description |
| --- | --- |
| `apiVersion` _string_ | `crd.questdb.io/v1beta1`
| `kind` _string_ | `QuestDBSnapshotSchedule`
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |
| `spec` _[QuestDBSnapshotScheduleSpec](#questdbsnapshotschedulespec)_ |  |
| `status` _[QuestDBSnapshotScheduleStatus](#questdbsnapshotschedulestatus)_ |  |


#### QuestDBSnapshotScheduleSpec



QuestDBSnapshotScheduleSpec defines the desired state of QuestDBSnapshotSchedule

_Appears in:_
- [QuestDBSnapshotSchedule](#questdbsnapshotschedule)

| Field | Description |
| --- | --- |
| `schedule` _string_ |  |
| `retention` _integer_ |  |
| `snapshot` _[QuestDBSnapshotSpec](#questdbsnapshotspec)_ |  |


#### QuestDBSnapshotScheduleStatus



QuestDBSnapshotStatus defines the observed state of QuestDBSnapshot

_Appears in:_
- [QuestDBSnapshotSchedule](#questdbsnapshotschedule)

| Field | Description |
| --- | --- |
| `lastSnapshot` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#time-v1-meta)_ |  |
| `snapshotPhase` _QuestDBSnapshotPhase_ |  |


#### QuestDBSnapshotSpec



QuestDBSnapshotSpec defines the desired state of QuestDBSnapshot

_Appears in:_
- [QuestDBSnapshot](#questdbsnapshot)
- [QuestDBSnapshotScheduleSpec](#questdbsnapshotschedulespec)

| Field | Description |
| --- | --- |
| `questdbName` _string_ |  |
| `volumeSnapshotClassName` _string_ |  |
| `jobBackoffLimit` _integer_ |  |


#### QuestDBSnapshotStatus



QuestDBSnapshotStatus defines the observed state of QuestDBSnapshot

_Appears in:_
- [QuestDBSnapshot](#questdbsnapshot)

| Field | Description |
| --- | --- |
| `phase` _QuestDBSnapshotPhase_ |  |


#### QuestDBSpec



QuestDBSpec defines the desired state of QuestDB

_Appears in:_
- [QuestDB](#questdb)

| Field | Description |
| --- | --- |
| `volume` _[QuestDBVolumeSpec](#questdbvolumespec)_ |  |
| `config` _[QuestDBConfigSpec](#questdbconfigspec)_ |  |
| `image` _string_ |  |
| `affinity` _[Affinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#affinity-v1-core)_ |  |
| `extraEnv` _[EnvVar](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#envvar-v1-core) array_ |  |
| `extraVolumeMounts` _[VolumeMount](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#volumemount-v1-core) array_ |  |
| `extraVolumes` _[Volume](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#volume-v1-core) array_ |  |
| `imagePullPolicy` _[PullPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#pullpolicy-v1-core)_ | ImagePullPolicy defaults to IfNotPresent |
| `imagePullSecrets` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#localobjectreference-v1-core) array_ |  |
| `nodeSelector` _object (keys:string, values:string)_ |  |
| `podAnnotations` _object (keys:string, values:string)_ |  |
| `podSecurityContext` _[PodSecurityContext](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#podsecuritycontext-v1-core)_ |  |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#resourcerequirements-v1-core)_ |  |
| `serviceAnnotations` _object (keys:string, values:string)_ |  |
| `statefulSetAnnotations` _object (keys:string, values:string)_ |  |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#toleration-v1-core) array_ |  |


#### QuestDBStatus



QuestDBStatus defines the observed state of QuestDB

_Appears in:_
- [QuestDB](#questdb)

| Field | Description |
| --- | --- |
| `statefulSetReadyReplicas` _integer_ |  |


#### QuestDBVolumeSpec





_Appears in:_
- [QuestDBSpec](#questdbspec)

| Field | Description |
| --- | --- |
| `selector` _[LabelSelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#labelselector-v1-meta)_ |  |
| `size` _[Quantity](https://kubernetes.io/docs/reference/kubernetes-api/common-definitions/quantity/)_ |  |
| `volumeName` _string_ |  |
| `storageClassName` _string_ |  |
| `snapshotName` _string_ |  |


