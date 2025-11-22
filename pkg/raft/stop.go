package raft

import (
	"context"
	"time"

	"go.uber.org/zap"
)

// Stop gracefully stops the Raft node
func (n *Node) Stop() error {
	n.logger.Info("Stopping Raft node")

	// Signal stop to all goroutines FIRST
	// This ensures RPC handlers will reject requests immediately
	n.mu.Lock()
	select {
	case <-n.stopCh:
		// Already stopped
		n.mu.Unlock()
		return nil
	default:
		close(n.stopCh)
	}
	// Set state to stopped to prevent any new operations
	n.state = StateFollower // Prevent becoming leader/candidate
	n.mu.Unlock()

	// Shutdown HTTP server (stops accepting new connections)
	// Existing connections will be closed gracefully
	if n.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := n.server.Shutdown(ctx); err != nil {
			n.logger.Error("Failed to shutdown server", zap.Error(err))
			// Force close if graceful shutdown fails
			if closeErr := n.server.Close(); closeErr != nil {
				n.logger.Error("Failed to force close server", zap.Error(closeErr))
			}
		}
	}

	// Give goroutines time to exit (especially apply loop)
	// Use a longer timeout to ensure apply loop finishes
	time.Sleep(500 * time.Millisecond)

	// Close apply channel only after goroutines have exited
	// Use applyChMu to ensure no one is accessing it
	n.applyChMu.Lock()
	if n.applyCh != nil && !n.applyChClosed {
		close(n.applyCh)
		n.applyChClosed = true
		// Don't set to nil - keep reference for IsStopped() checks
	}
	n.applyChMu.Unlock()

	n.logger.Info("Raft node stopped")
	return nil
}
