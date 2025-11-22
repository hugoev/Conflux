package raft

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"

	"go.uber.org/zap"
)

// StartTransport starts the HTTP transport for Raft RPCs
func (n *Node) StartTransport(listener net.Listener) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/raft/request_vote", n.handleRequestVoteHTTP)
	mux.HandleFunc("/raft/append_entries", n.handleAppendEntriesHTTP)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("Unmatched request: %s %s\n", r.Method, r.URL.Path)
		n.logger.Warn("Unmatched request", zap.String("path", r.URL.Path), zap.String("method", r.Method))
		http.NotFound(w, r)
	})

	// Configure server with timeouts for proper cleanup
	n.server = &http.Server{
		Handler:           mux,
		ReadTimeout:       5 * time.Second,
		WriteTimeout:      5 * time.Second,
		IdleTimeout:       10 * time.Second,
		ReadHeaderTimeout: 2 * time.Second,
	}

	// Start server in goroutine
	go func() {
		n.logger.Info("Starting Raft HTTP server", zap.String("addr", listener.Addr().String()))
		if err := n.server.Serve(listener); err != nil && err != http.ErrServerClosed {
			n.logger.Error("HTTP server error", zap.Error(err))
		}
	}()

	return nil
}

func (n *Node) handleRequestVoteHTTP(w http.ResponseWriter, r *http.Request) {
	var args RequestVoteArgs
	if err := json.NewDecoder(r.Body).Decode(&args); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var reply RequestVoteReply
	n.logger.Debug("Handling RequestVote", zap.Any("args", args))
	if err := n.RequestVote(&args, &reply); err != nil {
		n.logger.Error("Failed to process RequestVote", zap.Error(err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	n.logger.Debug("Handled RequestVote", zap.Any("args", args), zap.Any("reply", reply))

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(reply); err != nil {
		n.logger.Error("Failed to encode response", zap.Error(err))
	}
}

func (n *Node) handleAppendEntriesHTTP(w http.ResponseWriter, r *http.Request) {
	var args AppendEntriesArgs
	if err := json.NewDecoder(r.Body).Decode(&args); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var reply AppendEntriesReply
	n.AppendEntries(&args, &reply)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(reply); err != nil {
		n.logger.Error("Failed to encode response", zap.Error(err))
	}
}

// sendRequestVote sends RequestVote RPC to a peer
func (n *Node) sendRequestVote(peer string, args *RequestVoteArgs, reply *RequestVoteReply) error {
	// Peer is assumed to be "host:port"
	url := fmt.Sprintf("http://%s/raft/request_vote", peer)

	data, err := json.Marshal(args)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Connection", "close") // Ensure connection closes after request

	n.logger.Debug("Sending RequestVote", zap.String("peer", peer), zap.Int("term", args.Term))
	client := &http.Client{
		Timeout: 5000 * time.Millisecond,
		Transport: &http.Transport{
			DisableKeepAlives: true, // Don't reuse connections
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		n.logger.Warn("Failed to send RequestVote", zap.String("peer", peer), zap.Error(err))
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		n.logger.Warn("RequestVote returned bad status", zap.String("peer", peer), zap.Int("status", resp.StatusCode))
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return json.NewDecoder(resp.Body).Decode(reply)
}

// sendAppendEntries sends AppendEntries RPC to a peer
func (n *Node) sendAppendEntries(peer string, args *AppendEntriesArgs, reply *AppendEntriesReply) error {
	url := fmt.Sprintf("http://%s/raft/append_entries", peer)

	data, err := json.Marshal(args)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Connection", "close")

	client := &http.Client{
		Timeout: 5000 * time.Millisecond,
		Transport: &http.Transport{
			DisableKeepAlives: true,
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return json.NewDecoder(resp.Body).Decode(reply)
}
