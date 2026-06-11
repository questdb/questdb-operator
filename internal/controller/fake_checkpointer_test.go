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
	"sync"
)

// fakeCheckpointer is a test double for Checkpointer that records calls.
type fakeCheckpointer struct {
	mu           sync.Mutex
	createCalls  int
	releaseCalls int
	createErr    error
	releaseErr   error
}

func (f *fakeCheckpointer) Create(_ context.Context, _ QuestDBConn) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.createCalls++
	return f.createErr
}

func (f *fakeCheckpointer) Release(_ context.Context, _ QuestDBConn) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.releaseCalls++
	return f.releaseErr
}

func (f *fakeCheckpointer) creates() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.createCalls
}

func (f *fakeCheckpointer) releases() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.releaseCalls
}
