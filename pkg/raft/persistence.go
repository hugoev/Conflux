package raft

import (
	"fmt"
	"path/filepath"

	"github.com/hugovillarreal/conflux/pkg/snapshot"
	"github.com/hugovillarreal/conflux/pkg/wal"
	"go.uber.org/zap"
)

// InitializePersistence initializes WAL and snapshotter, and recovers state
func (n *Node) InitializePersistence() error {
	// Initialize WAL
	walDir := filepath.Join(n.dataDir, "wal")
	w, err := wal.New(walDir, n.logger)
	if err != nil {
		return fmt.Errorf("failed to create WAL: %w", err)
	}
	n.wal = w

	// Initialize snapshotter
	snapshotDir := filepath.Join(n.dataDir, "snapshots")
	s, err := snapshot.New(snapshotDir, n.logger)
	if err != nil {
		return fmt.Errorf("failed to create snapshotter: %w", err)
	}
	n.snapshotter = s

	// Recover state from snapshot and WAL
	if err := n.recoverFromDisk(); err != nil {
		return fmt.Errorf("failed to recover state: %w", err)
	}

	n.logger.Info("Initialized persistence",
		zap.String("dataDir", n.dataDir),
		zap.Int("logSize", len(n.log)),
		zap.Int("commitIndex", n.commitIndex))

	return nil
}

// recoverFromDisk recovers state from snapshot and WAL
func (n *Node) recoverFromDisk() error {
	n.mu.Lock()
	defer n.mu.Unlock()

	// Try to load snapshot
	snap, err := n.snapshotter.Load()
	if err != nil {
		return fmt.Errorf("failed to load snapshot: %w", err)
	}

	if snap != nil {
		// Apply snapshot
		n.logger.Info("Recovering from snapshot",
			zap.Int("lastIndex", snap.LastIncludedIndex),
			zap.Int("lastTerm", snap.LastIncludedTerm))

		// Truncate log and set indices
		n.log = []LogEntry{{Term: snap.LastIncludedTerm, Index: snap.LastIncludedIndex}}
		n.commitIndex = snap.LastIncludedIndex
		n.lastApplied = snap.LastIncludedIndex

		// Apply snapshot data to state machine
		// (This would be handled by the KV store layer)
	}

	// Read WAL entries
	entries, err := n.wal.ReadAll()
	if err != nil {
		return fmt.Errorf("failed to read WAL: %w", err)
	}

	if len(entries) > 0 {
		n.logger.Info("Recovering from WAL", zap.Int("entries", len(entries)))

		// Append entries that come after snapshot
		for _, walEntry := range entries {
			if walEntry.Index > n.commitIndex {
				// Convert wal.LogEntry to raft.LogEntry
				raftEntry := LogEntry{
					Index:   walEntry.Index,
					Term:    walEntry.Term,
					Command: walEntry.Command,
				}
				n.log = append(n.log, raftEntry)
			}
		}
	}

	return nil
}

// persistLogEntry writes a log entry to WAL
func (n *Node) persistLogEntry(entry LogEntry) error {
	if n.wal == nil {
		return nil // Persistence not initialized
	}

	walEntry := wal.LogEntry{
		Index:   entry.Index,
		Term:    entry.Term,
		Command: entry.Command,
	}

	return n.wal.Append(walEntry)
}

// shouldSnapshot determines if we should create a snapshot
//nolint:unused // Reserved for future use
func (n *Node) shouldSnapshot() bool {
	const snapshotThreshold = 10000 // Create snapshot every 10k entries
	return len(n.log) > snapshotThreshold
}

// createSnapshot creates a snapshot of the current state
//nolint:unused // Reserved for future use
func (n *Node) createSnapshot(stateData []byte) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	if len(n.log) == 0 {
		return nil
	}

	lastEntry := n.log[len(n.log)-1]

	snap := snapshot.Snapshot{
		LastIncludedIndex: lastEntry.Index,
		LastIncludedTerm:  lastEntry.Term,
		Data:              stateData,
	}

	if err := n.snapshotter.Save(snap); err != nil {
		return fmt.Errorf("failed to save snapshot: %w", err)
	}

	// Truncate WAL to save space
	if err := n.wal.Truncate(lastEntry.Index); err != nil {
		n.logger.Warn("Failed to truncate WAL", zap.Error(err))
	}

	// Compact log (keep only recent entries)
	const keepEntries = 100
	if len(n.log) > keepEntries {
		n.log = n.log[len(n.log)-keepEntries:]
	}

	n.logger.Info("Created snapshot",
		zap.Int("lastIndex", snap.LastIncludedIndex),
		zap.Int("lastTerm", snap.LastIncludedTerm),
		zap.Int("size", len(snap.Data)))

	return nil
}
