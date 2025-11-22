package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// KV metrics
	KVRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kv_requests_total",
			Help: "Total number of KV requests",
		},
		[]string{"method", "status"},
	)

	KVRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "kv_request_duration_seconds",
			Help:    "KV request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method"},
	)

	// Raft metrics
	RaftIsLeader = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "raft_is_leader",
			Help: "1 if this node is the leader, 0 otherwise",
		},
	)

	RaftTerm = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "raft_term",
			Help: "Current Raft term",
		},
	)

	RaftCommitIndex = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "raft_commit_index",
			Help: "Current commit index",
		},
	)

	RaftLogEntriesTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "raft_log_entries_total",
			Help: "Total number of log entries",
		},
	)

	RaftElectionTimeoutsTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "raft_election_timeouts_total",
			Help: "Total number of election timeouts",
		},
	)

	// Storage metrics
	WALSizeBytes = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "wal_size_bytes",
			Help: "Size of WAL in bytes",
		},
	)

	SnapshotSizeBytes = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "snapshot_size_bytes",
			Help: "Size of latest snapshot in bytes",
		},
	)
)
