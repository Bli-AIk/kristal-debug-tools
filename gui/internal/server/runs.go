// Run log: games and tasks are launched in a NEW terminal window (see
// internal/termrun) detached from the GUI, so this is just a history of
// what was started — no output capture, no cancellation.
package server

import (
	"sync"
	"time"
)

// runEntry is one history entry.
type runEntry struct {
	ID      int    `json:"id"`
	Label   string `json:"label"`
	Command string `json:"command"`
	Started string `json:"started"`
}

const maxLogEntries = 50

type runManager struct {
	mu   sync.Mutex
	log  []runEntry
	next int
}

func newRunManager() *runManager {
	return &runManager{}
}

// add records a started command and returns its id.
func (m *runManager) add(label, cmdStr string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	id := m.next
	m.next++
	m.log = append(m.log, runEntry{
		ID: id, Label: label, Command: cmdStr,
		Started: time.Now().Format(time.RFC3339),
	})
	if len(m.log) > maxLogEntries {
		m.log = m.log[len(m.log)-maxLogEntries:]
	}
	return id
}

func (m *runManager) logSnapshot() []runEntry {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]runEntry, len(m.log))
	copy(out, m.log)
	return out
}
