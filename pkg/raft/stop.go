package raft

import (
	"context"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
)

// Stop gracefully stops the Raft node
func (n *Node) Stop() error {
	n.logger.Info("Stopping Raft node")

	// Set stopped flag FIRST (atomic, no lock needed)
	// This ensures fast checks in RPC handlers without acquiring locks
	atomic.StoreInt32(&n.stopped, 1)

	// Signal stop to all goroutines
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
	// Set state to Follower and clear votedFor to prevent any election participation
	n.state = StateFollower
	n.votedFor = "" // Clear vote to prevent granting votes
	n.mu.Unlock()

	// Shutdown HTTP server IMMEDIATELY (stops accepting new connections)
	// This must happen right after closing stopCh to minimize race window
	if n.server != nil {
		// Use a very short timeout to fail fast
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		if err := n.server.Shutdown(ctx); err != nil {
			n.logger.Error("Failed to shutdown server gracefully", zap.Error(err))
			// Force close immediately if graceful shutdown fails
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
