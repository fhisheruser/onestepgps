// Package service holds the application's use cases. It depends on the domain
// ports only, which keeps it testable without a database, an HTTP server or a
// live GPS provider.
package service

import (
	"sync"

	"fleetview/internal/domain"
)

// SnapshotCache is a concurrency-safe holder for the most recent fleet
// snapshot. Exactly one writer (the poller) and many readers (HTTP handlers,
// WebSocket broadcasts) use it, which is what the RWMutex is optimised for.
//
// The Devices slice inside a snapshot is treated as immutable: the poller
// always publishes a freshly built slice and never mutates a published one, so
// readers can share it without copying.
type SnapshotCache struct {
	mu       sync.RWMutex
	snapshot domain.Snapshot
}

// NewSnapshotCache returns an empty cache.
func NewSnapshotCache() *SnapshotCache { return &SnapshotCache{} }

// Get returns the current snapshot. The returned Devices slice is read-only.
func (c *SnapshotCache) Get() domain.Snapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.snapshot
}

// Set publishes a new snapshot.
func (c *SnapshotCache) Set(s domain.Snapshot) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.snapshot = s
}

// MarkStale flags the cached data as no longer fresh and records why, keeping
// the last known good devices in place so the dashboard degrades gracefully.
func (c *SnapshotCache) MarkStale(reason string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.snapshot.Stale = true
	c.snapshot.Error = reason
}

var _ domain.SnapshotStore = (*SnapshotCache)(nil)
