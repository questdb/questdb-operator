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
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// QuestDBConn identifies a QuestDB pg-wire endpoint and its credentials.
type QuestDBConn struct {
	Host     string
	Port     int
	User     string
	Password string
}

// Checkpointer runs QuestDB CHECKPOINT SQL against a QuestDB. It is an interface so the backup
// controller can be unit-tested without a live database.
//
// QuestDB allows only one active checkpoint at a time. Create tolerates an "already exists"
// error (the checkpoint is active, which is what the caller wants); Release tolerates a
// "no active checkpoint" error (the obligation to release is already satisfied). Both make the
// operations effectively idempotent so a crashed-and-retried reconcile converges.
type Checkpointer interface {
	Create(ctx context.Context, target QuestDBConn) error
	Release(ctx context.Context, target QuestDBConn) error
}

// pgxCheckpointer is the production Checkpointer backed by a pg-wire connection.
type pgxCheckpointer struct{}

// NewCheckpointer returns the production Checkpointer.
func NewCheckpointer() Checkpointer { return pgxCheckpointer{} }

func (pgxCheckpointer) exec(ctx context.Context, target QuestDBConn, sql string) error {
	cfg, err := pgx.ParseConfig("")
	if err != nil {
		return err
	}
	cfg.Host = target.Host
	cfg.Port = uint16(target.Port)
	cfg.User = target.User
	cfg.Password = target.Password
	cfg.Database = "qdb"
	cfg.TLSConfig = nil // QuestDB OSS pg-wire does not offer TLS
	// QuestDB is most compatible with the simple query protocol for utility statements.
	cfg.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol

	conn, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close(ctx) }()

	_, err = conn.Exec(ctx, sql)
	return err
}

func (p pgxCheckpointer) Create(ctx context.Context, target QuestDBConn) error {
	err := p.exec(ctx, target, "CHECKPOINT CREATE")
	if err != nil && isCheckpointAlreadyExists(err) {
		return nil
	}
	return err
}

func (p pgxCheckpointer) Release(ctx context.Context, target QuestDBConn) error {
	err := p.exec(ctx, target, "CHECKPOINT RELEASE")
	if err != nil && isNoActiveCheckpoint(err) {
		return nil
	}
	return err
}

// isCheckpointAlreadyExists reports whether a CHECKPOINT CREATE error means a checkpoint is already
// active (so the caller may adopt it). Only a *server* response qualifies: a transport error
// (connection refused/reset, timeout) is never "already exists" and must propagate so the operation
// is retried. QuestDB 9.x reports an already-active checkpoint as a PgError with SQLSTATE 00000 and
// message "Waiting for CHECKPOINT RELEASE to be called" (verified against questdb/questdb:9.4.2);
// the extra phrasings keep this robust across versions.
func isCheckpointAlreadyExists(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	m := strings.ToLower(pgErr.Message)
	if strings.Contains(m, "waiting for checkpoint release") {
		return true
	}
	return strings.Contains(m, "checkpoint") &&
		(strings.Contains(m, "already") || strings.Contains(m, "in progress") || strings.Contains(m, "exists"))
}

// isNoActiveCheckpoint reports whether a CHECKPOINT RELEASE error means there was no checkpoint to
// release (so the release obligation is already satisfied). As above, only a server response
// qualifies — a transport error must propagate and be retried, never be mistaken for "nothing to
// release", which would strand an open checkpoint the operator believes it released. QuestDB 9.x
// simply returns success for a redundant RELEASE, so this is a cross-version safety net, not the
// common path; the phrases are specific enough not to match transport wording like "could not".
func isNoActiveCheckpoint(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	m := strings.ToLower(pgErr.Message)
	return strings.Contains(m, "checkpoint") &&
		(strings.Contains(m, "no active") || strings.Contains(m, "not active") ||
			strings.Contains(m, "no checkpoint") || strings.Contains(m, "without") ||
			strings.Contains(m, "never created") || strings.Contains(m, "was not created") ||
			strings.Contains(m, "not been created"))
}
