package wal

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"go.uber.org/zap"
)

// LogEntry represents a single log entry (imported from raft package)
type LogEntry struct {
	Index   int
	Term    int
	Command interface{}
}

// WAL represents a Write-Ahead Log
type WAL struct {
	dir     string
	file    *os.File
	encoder *json.Encoder
	mu      sync.Mutex
	logger  *zap.Logger
}

// New creates a new WAL in the specified directory
func New(dir string, logger *zap.Logger) (*WAL, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create WAL directory: %w", err)
	}

	walPath := filepath.Join(dir, "raft.wal")
	file, err := os.OpenFile(walPath, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open WAL file: %w", err)
	}

	return &WAL{
		dir:     dir,
		file:    file,
		encoder: json.NewEncoder(file),
		logger:  logger,
	}, nil
}

// Append writes a log entry to the WAL
func (w *WAL) Append(entry LogEntry) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if err := w.encoder.Encode(entry); err != nil {
		return fmt.Errorf("failed to encode entry: %w", err)
	}

	// Sync to disk for durability
	if err := w.file.Sync(); err != nil {
		return fmt.Errorf("failed to sync WAL: %w", err)
	}

	w.logger.Debug("Appended entry to WAL", zap.Int("index", entry.Index), zap.Int("term", entry.Term))
	return nil
}

// ReadAll reads all entries from the WAL
func (w *WAL) ReadAll() ([]LogEntry, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Seek to beginning
	if _, err := w.file.Seek(0, 0); err != nil {
		return nil, fmt.Errorf("failed to seek WAL: %w", err)
	}

	var entries []LogEntry
	decoder := json.NewDecoder(w.file)

	for {
		var entry LogEntry
		if err := decoder.Decode(&entry); err == io.EOF {
			break
		} else if err != nil {
			return nil, fmt.Errorf("failed to decode entry: %w", err)
		}
		entries = append(entries, entry)
	}

	w.logger.Info("Read entries from WAL", zap.Int("count", len(entries)))
	return entries, nil
}

// Truncate removes all entries after the specified index
func (w *WAL) Truncate(afterIndex int) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Read all entries
	if _, err := w.file.Seek(0, 0); err != nil {
		return fmt.Errorf("failed to seek WAL: %w", err)
	}

	var entries []LogEntry
	decoder := json.NewDecoder(w.file)

	for {
		var entry LogEntry
		if err := decoder.Decode(&entry); err == io.EOF {
			break
		} else if err != nil {
			return fmt.Errorf("failed to decode entry: %w", err)
		}
		if entry.Index <= afterIndex {
			entries = append(entries, entry)
		}
	}

	// Rewrite file with truncated entries
	if err := w.file.Truncate(0); err != nil {
		return fmt.Errorf("failed to truncate file: %w", err)
	}
	if _, err := w.file.Seek(0, 0); err != nil {
		return fmt.Errorf("failed to seek file: %w", err)
	}

	w.encoder = json.NewEncoder(w.file)
	for _, entry := range entries {
		if err := w.encoder.Encode(entry); err != nil {
			return fmt.Errorf("failed to re-encode entry: %w", err)
		}
	}

	if err := w.file.Sync(); err != nil {
		return fmt.Errorf("failed to sync after truncate: %w", err)
	}

	w.logger.Info("Truncated WAL", zap.Int("afterIndex", afterIndex), zap.Int("remaining", len(entries)))
	return nil
}

// Close closes the WAL
func (w *WAL) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if err := w.file.Sync(); err != nil {
		return fmt.Errorf("failed to sync before close: %w", err)
	}

	if err := w.file.Close(); err != nil {
		return fmt.Errorf("failed to close WAL: %w", err)
	}

	w.logger.Info("Closed WAL")
	return nil
}
