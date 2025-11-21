package raft

import "go.uber.org/zap"

// startApplyLoop runs a goroutine that applies committed entries to the state machine
func (n *Node) startApplyLoop() {
	go func() {
		for {
			select {
			case <-n.stopCh:
				return
			default:
			}

			n.mu.Lock()
			// Check if there are entries to apply
			if n.commitIndex > n.lastApplied {
				// Apply entries from lastApplied+1 to commitIndex
				entriesToApply := make([]LogEntry, 0)
				for i := n.lastApplied + 1; i <= n.commitIndex; i++ {
					if i < len(n.log) {
						entriesToApply = append(entriesToApply, n.log[i])
					}
				}
				n.lastApplied = n.commitIndex
				n.mu.Unlock()

				// Send to applyCh (without holding lock)
				for _, entry := range entriesToApply {
					msg := ApplyMsg{
						CommandValid: true,
						Command:      entry.Command,
						CommandIndex: entry.Index,
					}
					select {
					case n.applyCh <- msg:
						n.logger.Debug("Applied entry", zap.Int("index", entry.Index), zap.Int("term", entry.Term))
					case <-n.stopCh:
						return
					}
				}
			} else {
				n.mu.Unlock()
				// Sleep briefly to avoid busy-waiting
				select {
				case <-n.stopCh:
					return
				default:
				}
			}
		}
	}()
}
