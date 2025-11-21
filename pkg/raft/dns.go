package raft

import (
	"net"
	"strings"
	"time"

	"go.uber.org/zap"
)

// waitForDNSReady waits for peer DNS names to be resolvable before starting Raft
func (n *Node) waitForDNSReady() {
	n.logger.Info("Waiting for DNS to be ready for peers")

	maxWait := 60 * time.Second
	checkInterval := 2 * time.Second
	deadline := time.Now().Add(maxWait)

	for time.Now().Before(deadline) {
		allReady := true

		for _, peer := range n.peers {
			if peer == n.nodeID {
				continue
			}

			// Extract hostname from "host:port"
			host := peer
			if idx := strings.LastIndex(peer, ":"); idx != -1 {
				host = peer[:idx]
			}

			// Try to resolve the hostname
			_, err := net.LookupHost(host)
			if err != nil {
				n.logger.Debug("Peer DNS not ready yet", zap.String("peer", peer), zap.Error(err))
				allReady = false
				break
			}
		}

		if allReady {
			n.logger.Info("All peer DNS names resolved successfully")
			return
		}

		time.Sleep(checkInterval)
	}

	n.logger.Warn("DNS readiness check timed out, starting anyway", zap.Duration("waited", maxWait))
}
