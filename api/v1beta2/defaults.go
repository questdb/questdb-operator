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

const (
	// DefaultImage is the QuestDB OSS image used when QuestDB.spec.image is empty.
	DefaultImage = "questdb/questdb:9.4.2"

	// DefaultFSGroup is applied to the pod so the mounted data volume is writable by QuestDB.
	DefaultFSGroup int64 = 10001

	// EnvPgUser and EnvPgPassword are the env vars QuestDB reads as pg.user / pg.password. The
	// operator owns them (injected from the credentials Secret), so users may not set them via
	// spec.extraEnv — doing so would desync the pod from the operator's checkpoint connection.
	EnvPgUser     = "QDB_PG_USER"
	EnvPgPassword = "QDB_PG_PASSWORD"
)
