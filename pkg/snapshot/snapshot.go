package snapshot

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"go.uber.org/zap"
)

// Snapshot represents a point-in-time snapshot of the state machine
type Snapshot struct {
	LastIncludedIndex int         `json:"last_included_index"`
	LastIncludedTerm  int         `json:"last_included_term"`
	Data              []byte      `json:"data"`
	Metadata          interface{} `json:"metadata,omitempty"`
}

// Snapshotter manages snapshot creation and loading
type Snapshotter struct {
	dir    string
	mu     sync.Mutex
	logger *zap.Logger
}

// New creates a new Snapshotter
func New(dir string, logger *zap.Logger) (*Snapshotter, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create snapshot directory: %w", err)
	}

	return &Snapshotter{
		dir:    dir,
		logger: logger,
	}, nil
}

// Save writes a snapshot to disk
func (s *Snapshotter) Save(snapshot Snapshot) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Write to temporary file first
	tmpPath := filepath.Join(s.dir, "snapshot.tmp")
	file, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("failed to create temp snapshot file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	if err := encoder.Encode(snapshot); err != nil {
		return fmt.Errorf("failed to encode snapshot: %w", err)
	}

	if err := file.Sync(); err != nil {
		return fmt.Errorf("failed to sync snapshot: %w", err)
	}

	// Atomically rename to final location
	finalPath := filepath.Join(s.dir, "snapshot.json")
	if err := os.Rename(tmpPath, finalPath); err != nil {
		return fmt.Errorf("failed to rename snapshot: %w", err)
	}

	s.logger.Info("Saved snapshot",
		zap.Int("lastIndex", snapshot.LastIncludedIndex),
		zap.Int("lastTerm", snapshot.LastIncludedTerm),
		zap.Int("size", len(snapshot.Data)))

	return nil
}

// Load reads the latest snapshot from disk
func (s *Snapshotter) Load() (*Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	snapshotPath := filepath.Join(s.dir, "snapshot.json")

	// Check if snapshot exists
	if _, err := os.Stat(snapshotPath); os.IsNotExist(err) {
		return nil, nil // No snapshot exists
	}

	file, err := os.Open(snapshotPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open snapshot: %w", err)
	}
	defer file.Close()

	var snapshot Snapshot
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&snapshot); err != nil {
		return nil, fmt.Errorf("failed to decode snapshot: %w", err)
	}

	s.logger.Info("Loaded snapshot",
		zap.Int("lastIndex", snapshot.LastIncludedIndex),
		zap.Int("lastTerm", snapshot.LastIncludedTerm),
		zap.Int("size", len(snapshot.Data)))

	return &snapshot, nil
}

// Delete removes the snapshot file
func (s *Snapshotter) Delete() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	snapshotPath := filepath.Join(s.dir, "snapshot.json")
	if err := os.Remove(snapshotPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete snapshot: %w", err)
	}

	s.logger.Info("Deleted snapshot")
	return nil
}
