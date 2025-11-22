package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hugovillarreal/conflux/pkg/api"
	"github.com/hugovillarreal/conflux/pkg/config"
	"github.com/hugovillarreal/conflux/pkg/kv"
	"github.com/hugovillarreal/conflux/pkg/raft"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

func main() {
	// Initialize logger
	logger, err := zap.NewProduction()
	if err != nil {
		panic(fmt.Sprintf("failed to initialize logger: %v", err))
	}
	defer logger.Sync()

	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		logger.Fatal("failed to load config", zap.Error(err))
	}

	logger.Info("Starting Raft KV node",
		zap.String("node_id", cfg.NodeID),
		zap.Int("port", cfg.Port),
		zap.Bool("raft_enabled", cfg.EnableRaft),
	)

	// Create data directory
	if err := os.MkdirAll(cfg.DataDir, 0755); err != nil {
		logger.Fatal("failed to create data directory", zap.Error(err))
	}

	// Initialize KV store
	store := kv.NewStore()

	// Initialize Raft node (even if not enabled, for future use)
	var raftNode *raft.Node
	if cfg.EnableRaft {
		// Create Raft node
		raftNode = raft.NewNode(cfg.NodeID, cfg.Peers, cfg.DataDir, logger)

		// Initialize persistence
		if err := raftNode.InitializePersistence(); err != nil {
			logger.Fatal("Failed to initialize persistence", zap.Error(err))
		}

		raftNode.Start()

		// Create listener for Raft transport
		raftListener, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.RaftPort))
		if err != nil {
			logger.Fatal("Failed to bind Raft transport port", zap.Error(err))
		}

		if err := raftNode.StartTransport(raftListener); err != nil {
			logger.Fatal("Failed to start Raft transport", zap.Error(err))
		}
		logger.Info("Raft consensus enabled", zap.Strings("peers", cfg.Peers), zap.Int("raft_port", cfg.RaftPort))

		// Start applying committed entries
		go func() {
			for msg := range raftNode.ApplyCh() {
				if msg.CommandValid {
					if cmd, ok := msg.Command.(*kv.Command); ok {
						store.Apply(cmd)
					} else if cmdMap, ok := msg.Command.(map[string]interface{}); ok {
						// Handle map conversion (from JSON unmarshaling)
						cmd := &kv.Command{}
						if typeStr, ok := cmdMap["type"].(string); ok {
							cmd.Type = kv.CommandType(typeStr)
						}
						if key, ok := cmdMap["key"].(string); ok {
							cmd.Key = key
						}
						if value, ok := cmdMap["value"].(string); ok {
							cmd.Value = value
						}
						store.Apply(cmd)
					} else {
						logger.Error("Invalid command type received from Raft", zap.Any("command", msg.Command))
					}
				}
			}
		}()
	} else {
		logger.Info("Running in single-node mode (Raft disabled)")
	}

	// Initialize HTTP API server
	apiServer := api.NewServer(store, raftNode, logger)

	// Setup HTTP server with Prometheus metrics
	mux := http.NewServeMux()
	mux.Handle("/", apiServer)
	mux.Handle("/metrics", promhttp.Handler())

	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Port),
		Handler: mux,
	}

	// Start HTTP server
	go func() {
		logger.Info("HTTP server starting", zap.String("addr", httpServer.Addr))
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("HTTP server failed", zap.Error(err))
		}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	<-sigChan
	logger.Info("Shutting down...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		logger.Error("HTTP server shutdown error", zap.Error(err))
	}

	logger.Info("Shutdown complete")
}
