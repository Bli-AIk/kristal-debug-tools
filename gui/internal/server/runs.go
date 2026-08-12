// Run manager: spawns tasks (just recipes and the love launch), streams their
// output over SSE and keeps a ring log. All process-tree kills are handled by
// the build-tagged kill files.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"sync"
	"time"
)

// event is one SSE frame on /api/runs/{id}/stream.
type event struct {
	Stream  string `json:"stream,omitempty"` // "stdout" | "stderr"
	Text    string `json:"text,omitempty"`
	Type    string `json:"type,omitempty"` // "exit" | "error"
	Code    int    `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

// runEntry is the summary shown in the runs log.
type runEntry struct {
	ID       int    `json:"id"`
	Label    string `json:"label"`
	Command  string `json:"command"`
	Started  string `json:"started"`
	Duration string `json:"duration,omitempty"`
	Code     int    `json:"code"` // -1 while running
}

const (
	maxEvents     = 50000
	maxLogEntries = 50
)

type runState struct {
	id     int
	label  string
	cmdStr string
	start  time.Time

	cancel context.CancelFunc

	mu     sync.Mutex
	events []event
	subs   map[chan struct{}]struct{}
	closed bool
	code   int // -1 while running
}

// append records an event and wakes subscribers. Slow subscribers drop
// wake-ups (the buffer is only a hint; the SSE handler re-reads history).
func (rs *runState) append(ev event) {
	rs.mu.Lock()
	rs.events = append(rs.events, ev)
	if n := len(rs.events); n > maxEvents {
		rs.events = rs.events[n-maxEvents:]
	}
	for ch := range rs.subs {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
	rs.mu.Unlock()
}

// finish marks the run as complete and wakes subscribers one last time.
func (rs *runState) finish(code int) {
	rs.mu.Lock()
	rs.code = code
	rs.closed = true
	for ch := range rs.subs {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
	rs.mu.Unlock()
}

// subscribe returns the current event history plus a wake-up channel that
// fires whenever events are appended or the run finishes.
func (rs *runState) subscribe() ([]event, <-chan struct{}, func()) {
	rs.mu.Lock()
	ch := make(chan struct{}, 16)
	rs.subs[ch] = struct{}{}
	events := make([]event, len(rs.events))
	copy(events, rs.events)
	rs.mu.Unlock()
	return events, ch, func() {
		rs.mu.Lock()
		delete(rs.subs, ch)
		rs.mu.Unlock()
	}
}

func (rs *runState) snapshot() ([]event, int, bool) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	events := make([]event, len(rs.events))
	copy(events, rs.events)
	return events, rs.code, rs.closed
}

type runManager struct {
	mu   sync.Mutex
	runs map[int]*runState
	log  []runEntry
	next int
}

func newRunManager() *runManager {
	return &runManager{runs: map[int]*runState{}}
}

// prepareCmd wires process-tree kill support into a command: the child runs
// in its own process group (Unix) and cmd.Cancel / WaitDelay handle the
// cancel path (Go >= 1.20).
func prepareCmd(cmd *exec.Cmd) {
	setProcAttrs(cmd)
	cmd.Cancel = func() error { return killTree(cmd) }
	cmd.WaitDelay = 3 * time.Second
}

// startFromCmd launches argv (dir as working directory) with output streaming
// into a new run, and records the run in the ring log.
func (m *runManager) startFromCmd(label, cmdStr string, argv []string, dir string) (int, error) {
	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = dir
	prepareCmd(cmd)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return 0, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return 0, err
	}
	if err := cmd.Start(); err != nil {
		cancel()
		return 0, err
	}

	rs := &runState{
		id:     m.next,
		label:  label,
		cmdStr: cmdStr,
		start:  time.Now(),
		cancel: cancel,
		subs:   map[chan struct{}]struct{}{},
		code:   -1,
	}
	m.next++
	m.mu.Lock()
	m.runs[rs.id] = rs
	m.log = append(m.log, runEntry{
		ID: rs.id, Label: rs.label, Command: rs.cmdStr,
		Started: rs.start.Format(time.RFC3339), Code: -1,
	})
	if len(m.log) > maxLogEntries {
		m.log = m.log[len(m.log)-maxLogEntries:]
	}
	m.mu.Unlock()

	go m.pipe(rs, stdout, "stdout")
	go m.pipe(rs, stderr, "stderr")
	go m.wait(rs, cmd, cancel)
	return rs.id, nil
}

func (m *runManager) pipe(rs *runState, r io.Reader, stream string) {
	buf := make([]byte, 4096)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			rs.append(event{Stream: stream, Text: string(buf[:n])})
		}
		if err != nil {
			return
		}
	}
}

func (m *runManager) wait(rs *runState, cmd *exec.Cmd, cancel context.CancelFunc) {
	defer cancel()
	code := 0
	if err := cmd.Wait(); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			code = ee.ExitCode()
		} else {
			code = 1
		}
	}
	rs.append(event{Type: "exit", Code: code})
	rs.finish(code)

	m.mu.Lock()
	for i := range m.log {
		if m.log[i].ID == rs.id {
			m.log[i].Code = code
			m.log[i].Duration = time.Since(rs.start).Round(time.Millisecond).String()
			break
		}
	}
	m.mu.Unlock()
}

func (m *runManager) get(id int) (*runState, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	rs, ok := m.runs[id]
	return rs, ok
}

func (m *runManager) logSnapshot() []runEntry {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]runEntry, len(m.log))
	copy(out, m.log)
	return out
}

// --- HTTP handlers ---

func (m *runManager) stream(w http.ResponseWriter, r *http.Request, id int) {
	rs, ok := m.get(id)
	if !ok {
		http.Error(w, "unknown run", http.StatusNotFound)
		return
	}
	fl, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	fl.Flush()

	_, wake, unsub := rs.subscribe()
	defer unsub()
	index := 0
	for {
		// Copy pending events out from under the lock, then write.
		rs.mu.Lock()
		for index < len(rs.events) {
			writeSSE(w, fl, rs.events[index])
			index++
		}
		closed := rs.closed
		rs.mu.Unlock()
		if closed {
			return
		}
		select {
		case <-r.Context().Done():
			return
		case <-wake:
		}
	}
}

func writeSSE(w http.ResponseWriter, fl http.Flusher, ev event) {
	data, _ := json.Marshal(ev)
	fmt.Fprintf(w, "data: %s\n\n", data)
	fl.Flush()
}
