package kv

import (
	"testing"
)

func TestStore_Apply(t *testing.T) {
	store := NewStore()

	// Test PUT
	cmd := &Command{
		Type:  CommandPut,
		Key:   "test-key",
		Value: "test-value",
	}
	store.Apply(cmd)

	value, ok := store.Get("test-key")
	if !ok {
		t.Error("Key should exist after PUT")
	}
	if value != "test-value" {
		t.Errorf("Expected value 'test-value', got '%s'", value)
	}

	// Test DELETE
	delCmd := &Command{
		Type: CommandDelete,
		Key:  "test-key",
	}
	store.Apply(delCmd)

	_, ok = store.Get("test-key")
	if ok {
		t.Error("Key should not exist after DELETE")
	}
}

func TestStore_Size(t *testing.T) {
	store := NewStore()

	if store.Size() != 0 {
		t.Error("Empty store should have size 0")
	}

	store.Apply(&Command{Type: CommandPut, Key: "key1", Value: "value1"})
	store.Apply(&Command{Type: CommandPut, Key: "key2", Value: "value2"})

	if store.Size() != 2 {
		t.Errorf("Expected size 2, got %d", store.Size())
	}
}

