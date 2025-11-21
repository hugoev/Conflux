package kv

// CommandType represents the type of operation
type CommandType string

const (
	CommandPut    CommandType = "PUT"
	CommandDelete CommandType = "DELETE"
)

// Command represents a state machine command
type Command struct {
	Type  CommandType `json:"type"`
	Key   string      `json:"key"`
	Value string      `json:"value,omitempty"`
}

// IsReadOnly returns true if the command is read-only
func (c *Command) IsReadOnly() bool {
	return false // All our commands are writes for now
}

