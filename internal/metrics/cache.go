package metrics

import (
	"context"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

// TypeCount holds the artifact count for one artifact type.
type TypeCount struct {
	TypeName string `json:"typeName"`
	Count    int64  `json:"count"`
}

// Snapshot is a point-in-time view of registry aggregate metrics.
type Snapshot struct {
	CachedAt                 time.Time   `json:"cachedAt"`
	TotalArtifacts           int64       `json:"totalArtifacts"`
	TotalVersions            int64       `json:"totalVersions"`
	TotalTags                int64       `json:"totalTags"`
	FilesAvailable           int64       `json:"filesAvailable"`
	FilesPending             int64       `json:"filesPending"`
	FilesError               int64       `json:"filesError"`
	TotalStorageBytes        int64       `json:"totalStorageBytes"`
	ArtifactsWithoutTags     int64       `json:"artifactsWithoutTags"`
	ArtifactsWithoutVersions int64       `json:"artifactsWithoutVersions"`
	TypeCounts               []TypeCount `json:"typeCounts"`
}

// Cache is a TTL-based in-memory cache for a single Snapshot.
// Concurrent requests after TTL expiry are collapsed via singleflight
// so only one DB refresh runs at a time.
type Cache struct {
	ttl   time.Duration
	mu    sync.RWMutex
	snap  *Snapshot
	group singleflight.Group
}

func NewCache(ttl time.Duration) *Cache {
	return &Cache{ttl: ttl}
}

// Get returns the cached snapshot if still fresh, otherwise calls load exactly
// once (even under concurrent requests) and caches the result.
func (c *Cache) Get(ctx context.Context, load func(context.Context) (*Snapshot, error)) (*Snapshot, error) {
	c.mu.RLock()
	if c.snap != nil && time.Since(c.snap.CachedAt) < c.ttl {
		s := c.snap
		c.mu.RUnlock()
		return s, nil
	}
	c.mu.RUnlock()

	v, err, _ := c.group.Do("metrics", func() (any, error) {
		// Use a detached context so cancellation of the triggering request
		// does not abort a load that other concurrent callers are waiting on.
		s, err := load(context.Background())
		if err != nil {
			return nil, err
		}
		c.mu.Lock()
		c.snap = s
		c.mu.Unlock()
		return s, nil
	})
	if err != nil {
		return nil, err
	}
	return v.(*Snapshot), nil
}
