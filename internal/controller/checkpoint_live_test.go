//go:build integration

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
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestLiveCheckpoint exercises the production Checkpointer against a real QuestDB reachable at
// QUESTDB_ADDR (host:port). It is build-tagged "integration" so it never runs under `make test`.
//
//	go test -tags=integration -run TestLiveCheckpoint ./internal/controller/ -count=1 -v
func TestLiveCheckpoint(t *testing.T) {
	addr := os.Getenv("QUESTDB_ADDR")
	if addr == "" {
		t.Skip("set QUESTDB_ADDR=host:port to run the live checkpoint test")
	}
	host, portStr, _ := strings.Cut(addr, ":")
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("invalid QUESTDB_ADDR %q: %v", addr, err)
	}
	conn := QuestDBConn{
		Host:     host,
		Port:     port,
		User:     envOr("QUESTDB_USER", "admin"),
		Password: envOr("QUESTDB_PASSWORD", "quest"),
	}
	cp := NewCheckpointer()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := cp.Create(ctx, conn); err != nil {
		t.Fatalf("CHECKPOINT CREATE failed: %v", err)
	}
	t.Log("CHECKPOINT CREATE ok")

	// A second CREATE must be tolerated (QuestDB allows only one active checkpoint); this also
	// validates the isCheckpointAlreadyExists heuristic against QuestDB's real error message.
	if err := cp.Create(ctx, conn); err != nil {
		t.Fatalf("second CHECKPOINT CREATE not tolerated (already-exists heuristic miss?): %v", err)
	}
	t.Log("second CHECKPOINT CREATE tolerated (already active)")

	if err := cp.Release(ctx, conn); err != nil {
		t.Fatalf("CHECKPOINT RELEASE failed: %v", err)
	}
	t.Log("CHECKPOINT RELEASE ok")

	// A second RELEASE must be tolerated (no active checkpoint); validates the
	// isNoActiveCheckpoint heuristic against QuestDB's real error message.
	if err := cp.Release(ctx, conn); err != nil {
		t.Fatalf("second CHECKPOINT RELEASE not tolerated (no-active heuristic miss?): %v", err)
	}
	t.Log("second CHECKPOINT RELEASE tolerated (none active)")
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
