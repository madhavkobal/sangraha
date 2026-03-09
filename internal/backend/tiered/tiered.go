// Package tiered implements a multi-tier storage backend. Objects are written to
// the hot tier first. A background worker periodically promotes/demotes objects
// between tiers based on access frequency and age rules.
package tiered

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/madhavkobal/sangraha/internal/backend"
	"github.com/madhavkobal/sangraha/internal/backend/localfs"
)

// TierConfig describes a single storage tier.
type TierConfig struct {
	// Name is a human-readable label (e.g. "hot", "warm", "cold").
	Name string `yaml:"name" mapstructure:"name"`
	// Backend is the underlying backend type: "localfs" (only supported value currently).
	Backend string `yaml:"backend" mapstructure:"backend"`
	// DataDir is the root directory for this tier's localfs backend.
	DataDir string `yaml:"data_dir" mapstructure:"data_dir"`
	// DemoteAfter is the duration after last access before demotion to the next tier.
	// Zero means objects in this tier are never automatically demoted.
	DemoteAfter time.Duration `yaml:"demote_after" mapstructure:"demote_after"`
	// MaxSize is the maximum bytes this tier may hold (0 = unlimited).
	MaxSize int64 `yaml:"max_size" mapstructure:"max_size"`
}

// accessRecord tracks when an object was last accessed for demotion decisions.
type accessRecord struct {
	lastAccess time.Time
	tier       int // index into TieredBackend.tiers
}

// TieredBackend is a backend.Backend that spreads objects across ordered tiers
// (hot → warm → cold). Reads promote objects back to the hot tier.
type TieredBackend struct {
	tiers   []backend.Backend
	configs []TierConfig
	mu      sync.RWMutex
	// access tracks per-object last-access time and tier location.
	access map[string]*accessRecord // key: "bucket/key"
}

// New creates a TieredBackend from an ordered slice of TierConfig values.
// At least one tier is required. The first tier is the hot tier.
func New(tiers []TierConfig) (*TieredBackend, error) {
	if len(tiers) == 0 {
		return nil, fmt.Errorf("tiered: at least one tier is required")
	}
	backends := make([]backend.Backend, 0, len(tiers))
	for i, tc := range tiers {
		switch tc.Backend {
		case "localfs", "":
			if tc.DataDir == "" {
				return nil, fmt.Errorf("tiered: tier %d (%q): data_dir must not be empty", i, tc.Name)
			}
			b, err := localfs.New(tc.DataDir)
			if err != nil {
				return nil, fmt.Errorf("tiered: tier %d (%q): %w", i, tc.Name, err)
			}
			backends = append(backends, b)
		default:
			return nil, fmt.Errorf("tiered: tier %d (%q): unsupported backend %q", i, tc.Name, tc.Backend)
		}
	}
	return &TieredBackend{
		tiers:   backends,
		configs: tiers,
		access:  make(map[string]*accessRecord),
	}, nil
}

// Write stores the object in the hot (first) tier.
func (t *TieredBackend) Write(ctx context.Context, bucket, key string, r io.Reader, size int64) (int64, error) {
	n, err := t.tiers[0].Write(ctx, bucket, key, r, size)
	if err != nil {
		return 0, err
	}
	t.mu.Lock()
	t.access[bucket+"/"+key] = &accessRecord{lastAccess: time.Now(), tier: 0}
	t.mu.Unlock()
	return n, nil
}

// Read retrieves the object from whichever tier contains it. If the object is
// found in a lower tier it is promoted back to the hot tier.
func (t *TieredBackend) Read(ctx context.Context, bucket, key string, w io.Writer) error {
	for i, b := range t.tiers {
		ok, err := b.Exists(ctx, bucket, key)
		if err != nil || !ok {
			continue
		}
		if i == 0 {
			// Already in hot tier — read directly.
			t.recordAccess(bucket, key, 0)
			return b.Read(ctx, bucket, key, w)
		}
		// Object is in a lower tier — promote to hot tier then read.
		if perr := t.promote(ctx, bucket, key, i); perr != nil {
			log.Warn().Err(perr).
				Str("bucket", bucket).Str("key", key).
				Int("from_tier", i).Msg("tiered: promotion failed; reading from lower tier")
			t.recordAccess(bucket, key, i)
			return b.Read(ctx, bucket, key, w)
		}
		t.recordAccess(bucket, key, 0)
		return t.tiers[0].Read(ctx, bucket, key, w)
	}
	return &backend.ErrNotFound{Bucket: bucket, Key: key}
}

// Delete removes the object from all tiers.
func (t *TieredBackend) Delete(ctx context.Context, bucket, key string) error {
	var lastErr error
	for _, b := range t.tiers {
		if err := b.Delete(ctx, bucket, key); err != nil {
			lastErr = err
		}
	}
	t.mu.Lock()
	delete(t.access, bucket+"/"+key)
	t.mu.Unlock()
	return lastErr
}

// Exists checks if the object exists in any tier.
func (t *TieredBackend) Exists(ctx context.Context, bucket, key string) (bool, error) {
	for _, b := range t.tiers {
		ok, err := b.Exists(ctx, bucket, key)
		if err != nil {
			return false, err
		}
		if ok {
			return true, nil
		}
	}
	return false, nil
}

// Stat returns metadata from whichever tier contains the object.
func (t *TieredBackend) Stat(ctx context.Context, bucket, key string) (backend.ObjectInfo, error) {
	for _, b := range t.tiers {
		ok, err := b.Exists(ctx, bucket, key)
		if err != nil {
			return backend.ObjectInfo{}, err
		}
		if ok {
			return b.Stat(ctx, bucket, key)
		}
	}
	return backend.ObjectInfo{}, &backend.ErrNotFound{Bucket: bucket, Key: key}
}

// RunDemotionWorker starts a background goroutine that periodically checks
// objects in each tier and demotes them to the next tier when DemoteAfter has
// elapsed since their last access.
func (t *TieredBackend) RunDemotionWorker(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				t.runDemotion(ctx)
			}
		}
	}()
}

// runDemotion scans in-memory access records and demotes cold objects.
func (t *TieredBackend) runDemotion(ctx context.Context) {
	t.mu.RLock()
	snapshot := make(map[string]*accessRecord, len(t.access))
	for k, v := range t.access {
		snapshot[k] = v
	}
	t.mu.RUnlock()

	for objKey, rec := range snapshot {
		currentTier := rec.tier
		// Determine the DemoteAfter for the current tier.
		if currentTier >= len(t.configs)-1 {
			continue // already in coldest tier
		}
		cfg := t.configs[currentTier]
		if cfg.DemoteAfter == 0 {
			continue // this tier has no demotion policy
		}
		if time.Since(rec.lastAccess) < cfg.DemoteAfter {
			continue // not cold enough yet
		}

		// Parse bucket and key from the compound key.
		bucket, key := splitObjKey(objKey)
		if bucket == "" || key == "" {
			continue
		}

		targetTier := currentTier + 1
		if err := t.demote(ctx, bucket, key, currentTier, targetTier); err != nil {
			log.Warn().Err(err).
				Str("bucket", bucket).Str("key", key).
				Int("from_tier", currentTier).Int("to_tier", targetTier).
				Msg("tiered: demotion failed")
			continue
		}
		log.Debug().
			Str("bucket", bucket).Str("key", key).
			Str("from", t.configs[currentTier].Name).
			Str("to", t.configs[targetTier].Name).
			Msg("tiered: demoted object")

		t.mu.Lock()
		if ar, ok := t.access[objKey]; ok {
			ar.tier = targetTier
		}
		t.mu.Unlock()
	}
}

// promote copies an object from tier srcIdx to tier 0 (hot), then deletes it from srcIdx.
func (t *TieredBackend) promote(ctx context.Context, bucket, key string, srcIdx int) error {
	return t.moveObject(ctx, bucket, key, srcIdx, 0)
}

// demote copies an object from tier srcIdx to tier dstIdx, then deletes it from srcIdx.
func (t *TieredBackend) demote(ctx context.Context, bucket, key string, srcIdx, dstIdx int) error {
	return t.moveObject(ctx, bucket, key, srcIdx, dstIdx)
}

// moveObject copies object data from src tier to dst tier, then removes it from src.
func (t *TieredBackend) moveObject(ctx context.Context, bucket, key string, srcIdx, dstIdx int) error {
	src := t.tiers[srcIdx]
	dst := t.tiers[dstIdx]

	// Buffer the object — backends may not support concurrent read+write.
	var buf bytes.Buffer
	if err := src.Read(ctx, bucket, key, &buf); err != nil {
		return fmt.Errorf("tiered move: read from tier %d: %w", srcIdx, err)
	}
	size := int64(buf.Len())
	if _, err := dst.Write(ctx, bucket, key, &buf, size); err != nil {
		return fmt.Errorf("tiered move: write to tier %d: %w", dstIdx, err)
	}
	if err := src.Delete(ctx, bucket, key); err != nil {
		return fmt.Errorf("tiered move: delete from tier %d: %w", srcIdx, err)
	}
	return nil
}

// recordAccess updates the in-memory last-access time for an object.
func (t *TieredBackend) recordAccess(bucket, key string, tierIdx int) {
	k := bucket + "/" + key
	t.mu.Lock()
	if ar, ok := t.access[k]; ok {
		ar.lastAccess = time.Now()
		ar.tier = tierIdx
	} else {
		t.access[k] = &accessRecord{lastAccess: time.Now(), tier: tierIdx}
	}
	t.mu.Unlock()
}

// splitObjKey splits a "bucket/key" compound string back into bucket and key.
// The key portion may itself contain "/" characters.
func splitObjKey(compound string) (bucket, key string) {
	for i, c := range compound {
		if c == '/' {
			return compound[:i], compound[i+1:]
		}
	}
	return "", ""
}
