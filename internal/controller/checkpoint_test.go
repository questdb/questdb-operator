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
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

// pgErr builds a server-side PgError carrying the given message.
func pgErr(code, msg string) error {
	return &pgconn.PgError{Code: code, Message: msg}
}

// TestCheckpointLeaseOutlivesHoldDeadline pins the invariant that the checkpoint lease cannot expire
// before a holder's open checkpoint would be force-released by its hold-deadline abort. If the lease
// expired first, a sibling backup could take the slot over and open a second checkpoint on the same
// QuestDB concurrently.
func TestCheckpointLeaseOutlivesHoldDeadline(t *testing.T) {
	if checkpointLeaseDuration <= checkpointHoldDeadline {
		t.Fatalf("checkpointLeaseDuration (%s) must exceed checkpointHoldDeadline (%s)",
			checkpointLeaseDuration, checkpointHoldDeadline)
	}
}

func TestIsCheckpointAlreadyExists(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		// Verified against questdb/questdb:9.4.2: a second CHECKPOINT CREATE returns this.
		{"questdb 9.x already-active", pgErr("00000", "Waiting for CHECKPOINT RELEASE to be called"), true},
		{"generic already", pgErr("XX000", "checkpoint already in progress"), true},
		// A transport error is NOT a server "already exists" and must propagate so we retry.
		{"transport could-not-connect", errors.New("failed to connect: dial tcp: connection refused"), false},
		{"transport mentioning checkpoint", fmt.Errorf("checkpoint: could not send query: connection reset"), false},
		{"unrelated server error", pgErr("42601", "syntax error"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isCheckpointAlreadyExists(tc.err); got != tc.want {
				t.Fatalf("isCheckpointAlreadyExists(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

func TestIsNoActiveCheckpoint(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"server no active", pgErr("XX000", "no active checkpoint to release"), true},
		{"server not been created", pgErr("XX000", "checkpoint has not been created"), true},
		// The critical regression: a transport failure must never be read as "nothing to release",
		// which would let the controller record the checkpoint released while it is still open.
		{"transport could-not-connect", errors.New("failed to connect: dial tcp: connection refused"), false},
		{"transport checkpoint could-not", fmt.Errorf("checkpoint release: could not send query: connection reset"), false},
		{"unrelated server error", pgErr("42601", "syntax error"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isNoActiveCheckpoint(tc.err); got != tc.want {
				t.Fatalf("isNoActiveCheckpoint(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}
