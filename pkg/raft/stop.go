package raft

import (
	"context"
	"time"

	"go.uber.org/zap"
)

// Stop gracefully stops the Raft node
func (n *Node) Stop() error {
	n.logger.Info("Stopping Raft node")

	// Signal stop to all goroutines
	select {
	case <-n.stopCh:
		// Already stopped
		return nil
	default:
		close(n.stopCh)
	}

	// Shutdown HTTP server
	if n.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := n.server.Shutdown(ctx); err != nil {
			n.logger.Error("Failed to shutdown server", zap.Error(err))
			return err
		}
	}

	// Give goroutines time to exit
	time.Sleep(100 * time.Millisecond)

	// Close apply channel
	close(n.applyCh)

	n.logger.Info("Raft node stopped")
	return nil
}
