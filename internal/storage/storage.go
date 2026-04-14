package storage

import "io"

// Storage is the artifact persistence abstraction.
type Storage interface {
	// Store writes data and returns the resolved storage path.
	Store(suggestedPath string, data io.Reader, sizeHint int64, contentType string) (string, error)
	// Retrieve opens the stored artifact for streaming. Caller must close.
	Retrieve(storagePath string) (io.ReadCloser, error)
	// Delete removes the artifact. Idempotent.
	Delete(storagePath string) error
}
