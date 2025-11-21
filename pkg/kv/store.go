package kv

import (
	"sync"
)

// Store is the key-value state machine
type Store struct {
	mu   sync.RWMutex
	data map[string]string
}

// NewStore creates a new key-value store
func NewStore() *Store {
	return &Store{
		data: make(map[string]string),
	}
}

// Apply applies a command to the state machine
func (s *Store) Apply(cmd *Command) interface{} {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch cmd.Type {
	case CommandPut:
		s.data[cmd.Key] = cmd.Value
		return nil
	case CommandDelete:
		delete(s.data, cmd.Key)
		return nil
	default:
		return nil
	}
}

// Get retrieves a value by key (read-only, doesn't go through Raft)
func (s *Store) Get(key string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, ok := s.data[key]
	return value, ok
}

// GetAll returns all key-value pairs (for debugging)
func (s *Store) GetAll() map[string]string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make(map[string]string)
	for k, v := range s.data {
		result[k] = v
	}
	return result
}

// Size returns the number of keys in the store
func (s *Store) Size() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.data)
}

