package entities

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type runtimeRegistry struct {
	path string

	mu    sync.Mutex
	kinds map[string]runtimeEntityKind
}

func newRuntimeRegistry(path string) (*runtimeRegistry, error) {
	registry := &runtimeRegistry{
		path:  path,
		kinds: make(map[string]runtimeEntityKind),
	}
	if path == "" {
		return registry, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return registry, nil
		}
		return nil, fmt.Errorf("read registry file: %w", err)
	}
	if len(data) == 0 {
		return registry, nil
	}

	if err := json.Unmarshal(data, &registry.kinds); err != nil {
		return nil, fmt.Errorf("decode registry file: %w", err)
	}
	return registry, nil
}

func (r *runtimeRegistry) Persistent() bool {
	return r.path != ""
}

func (r *runtimeRegistry) Upsert(key string, kind runtimeEntityKind) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.kinds[key] = kind
	return r.save()
}

func (r *runtimeRegistry) Remove(key string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.kinds, key)
	return r.save()
}

func (r *runtimeRegistry) Kind(key string) (runtimeEntityKind, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	kind, ok := r.kinds[key]
	return kind, ok
}

func (r *runtimeRegistry) Snapshot() map[string]runtimeEntityKind {
	r.mu.Lock()
	defer r.mu.Unlock()

	out := make(map[string]runtimeEntityKind, len(r.kinds))
	for key, kind := range r.kinds {
		out[key] = kind
	}
	return out
}

func (r *runtimeRegistry) save() error {
	if r.path == "" {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(r.path), 0o755); err != nil {
		return fmt.Errorf("create registry directory: %w", err)
	}

	data, err := json.MarshalIndent(r.kinds, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal registry: %w", err)
	}

	tmpPath := r.path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		return fmt.Errorf("write registry temp file: %w", err)
	}
	if err := os.Rename(tmpPath, r.path); err != nil {
		return fmt.Errorf("replace registry file: %w", err)
	}
	return nil
}
