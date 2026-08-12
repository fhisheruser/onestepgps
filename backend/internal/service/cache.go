
package service

import (
	"sync"

	"fleetview/internal/domain"
)


type SnapshotCache struct {
	mu       sync.RWMutex
	snapshot domain.Snapshot
}


func NewSnapshotCache() *SnapshotCache { return &SnapshotCache{} }

func (c *SnapshotCache) Get() domain.Snapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.snapshot
}

func (c *SnapshotCache) Set(s domain.Snapshot) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.snapshot = s
}


func (c *SnapshotCache) MarkStale(reason string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.snapshot.Stale = true
	c.snapshot.Error = reason
}

var _ domain.SnapshotStore = (*SnapshotCache)(nil)
