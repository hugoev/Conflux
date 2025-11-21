package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/hugovillarreal/conflux/pkg/kv"
	"github.com/hugovillarreal/conflux/pkg/metrics"
	"github.com/hugovillarreal/conflux/pkg/raft"
	"go.uber.org/zap"
)

// Server handles HTTP API requests
type Server struct {
	router   *mux.Router
	store    *kv.Store
	logger   *zap.Logger
	raftNode *raft.Node
}

// NewServer creates a new HTTP API server
func NewServer(store *kv.Store, raftNode *raft.Node, logger *zap.Logger) *Server {
	s := &Server{
		router:   mux.NewRouter(),
		store:    store,
		raftNode: raftNode,
		logger:   logger,
	}
	s.setupRoutes()
	return s
}

func (s *Server) setupRoutes() {
	s.router.HandleFunc("/healthz", s.handleHealth).Methods("GET")
	s.router.HandleFunc("/metrics", s.handleMetrics).Methods("GET")
	s.router.HandleFunc("/kv/{key}", s.handleGet).Methods("GET")
	s.router.HandleFunc("/kv", s.handlePut).Methods("PUT")
	s.router.HandleFunc("/kv/{key}", s.handlePut).Methods("PUT")
	s.router.HandleFunc("/kv/{key}", s.handleDelete).Methods("DELETE")
}

// ServeHTTP implements http.Handler
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "healthy",
	})
}

func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	// Prometheus metrics are exposed via the default registry
	// This endpoint is for custom metrics if needed
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"store_size": s.store.Size(),
	})
}

func (s *Server) handleGet(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	vars := mux.Vars(r)
	key := vars["key"]

	value, ok := s.store.Get(key)
	if !ok {
		metrics.KVRequestsTotal.WithLabelValues("GET", "404").Inc()
		metrics.KVRequestDuration.WithLabelValues("GET").Observe(time.Since(start).Seconds())
		http.Error(w, "key not found", http.StatusNotFound)
		return
	}

	metrics.KVRequestsTotal.WithLabelValues("GET", "200").Inc()
	metrics.KVRequestDuration.WithLabelValues("GET").Observe(time.Since(start).Seconds())

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"key":   key,
		"value": value,
	})
}

func (s *Server) handlePut(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	var req struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		metrics.KVRequestsTotal.WithLabelValues("PUT", "400").Inc()
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Check if key is in URL
	vars := mux.Vars(r)
	if key, ok := vars["key"]; ok {
		req.Key = key
	}

	if req.Key == "" {
		http.Error(w, "key is required", http.StatusBadRequest)
		return
	}

	// For MVP v0, apply directly to store
	// In MVP v1+, this will go through Raft
	cmd := &kv.Command{
		Type:  kv.CommandPut,
		Key:   req.Key,
		Value: req.Value,
	}

	if s.raftNode != nil {
		if err := s.raftNode.Propose(cmd); err != nil {
			metrics.KVRequestsTotal.WithLabelValues("PUT", "500").Inc()
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		// In a real system, we'd wait for commit. For MVP, we assume success.
	} else {
		s.store.Apply(cmd)
	}

	metrics.KVRequestsTotal.WithLabelValues("PUT", "200").Inc()
	metrics.KVRequestDuration.WithLabelValues("PUT").Observe(time.Since(start).Seconds())

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
	})
}

func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	vars := mux.Vars(r)
	key := vars["key"]

	// For MVP v0, apply directly to store
	// In MVP v1+, this will go through Raft
	cmd := &kv.Command{
		Type: kv.CommandDelete,
		Key:  key,
	}

	if s.raftNode != nil {
		if err := s.raftNode.Propose(cmd); err != nil {
			metrics.KVRequestsTotal.WithLabelValues("DELETE", "500").Inc()
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	} else {
		s.store.Apply(cmd)
	}

	metrics.KVRequestsTotal.WithLabelValues("DELETE", "200").Inc()
	metrics.KVRequestDuration.WithLabelValues("DELETE").Observe(time.Since(start).Seconds())

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
	})
}
