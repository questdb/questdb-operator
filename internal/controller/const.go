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

import crdv1beta2 "github.com/questdb/questdb-operator/api/v1beta2"

const (
	// containerName is the name of the QuestDB container in the pod.
	containerName = "questdb"

	// QuestDB container filesystem layout (official Docker image).
	dataDir       = "/var/lib/questdb"      // install/server root: holds db/, conf/, .checkpoint/
	confDir       = "/var/lib/questdb/conf" // server.conf, log.conf, mime.types
	restoreMarker = "/var/lib/questdb/_restore"
	// restoreSentinelPrefix + the snapshot name gives a per-restore sentinel that persists on the
	// data volume, so the restore marker is created at most once per (volume, snapshot) pair.
	// Keying on the snapshot name — rather than a fixed filename — is deliberate: a fixed sentinel
	// would itself be captured inside any VolumeSnapshot taken after a restore, so restoring a
	// backup of an already-restored instance would find the sentinel and silently skip recovery.
	// A new restore always references a new snapshot name, so its sentinel is guaranteed absent.
	restoreSentinelPrefix = "/var/lib/questdb/.operator_restore_"

	// QuestDB OSS network ports.
	portHTTP   = 9000 // REST API + Web Console + ILP over HTTP
	portPgWire = 8812 // PostgreSQL wire protocol
	portILP    = 9009 // InfluxDB Line Protocol over TCP
	portHealth = 9003 // minimal HTTP server: health + Prometheus metrics

	// pg-wire credential secret keys (QuestDB reads these as env vars: pg.user / pg.password).
	envPgUser     = crdv1beta2.EnvPgUser
	envPgPassword = crdv1beta2.EnvPgPassword
	defaultPgUser = "admin"

	// CredentialsHashAnnotation carries a hash of the effective credentials on the pod template,
	// so rotating the referenced/generated Secret rolls the StatefulSet automatically.
	CredentialsHashAnnotation = "crd.questdb.io/credentials-hash"
	// ConfigHashAnnotation carries a hash of the rendered config so config changes roll the pod.
	ConfigHashAnnotation = "crd.questdb.io/config-hash"

	// credentialsSecretField indexes QuestDBs by their referenced credentials Secret name,
	// so a Secret change enqueues only the QuestDBs that use it (cached, indexed lookup).
	credentialsSecretField = ".spec.auth.psql.secretName"
)
