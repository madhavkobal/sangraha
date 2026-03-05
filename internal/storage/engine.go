// Package storage implements the core storage engine that orchestrates the
// backend (object data) and metadata store (bucket/object records).
package storage

import (
	"github.com/madhavkobal/sangraha/internal/backend"
	"github.com/madhavkobal/sangraha/internal/metadata"
)

// Engine orchestrates all storage subsystems. The zero value is not usable;
// construct with New.
type Engine struct {
	backend backend.Backend
	meta    metadata.Store
	ownerID string // ID of the root owner, used as default owner for buckets
}

// New creates a new storage Engine from the given backend and metadata store.
func New(b backend.Backend, m metadata.Store, ownerID string) *Engine {
	return &Engine{
		backend: b,
		meta:    m,
		ownerID: ownerID,
	}
}
