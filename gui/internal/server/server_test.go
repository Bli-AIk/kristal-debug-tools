package server

import (
	"bufio"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

func newTestServer(t *testing.T, opts Options) (*Server, *httptest.Server) {
	t.Helper()
	s := New(opts)
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)
	return s, ts
}

// startRun posts a run and returns its id.
func startRun(t *testing.T, ts *httptest.Server, body string) int {
	t.Helper()
	resp, err := http.Post(ts.URL+"/api/runs", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /api/runs: %s", resp.Status)
	}
	var out struct {
		ID int `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	return out.ID
}

// streamRun reads the SSE stream until it closes, returning all events.
func streamRun(t *testing.T, ts *httptest.Server, id int) []event {
	t.Helper()
	resp, err := http.Get(ts.URL + "/api/runs/" + itoa(id) + "/stream")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET stream: %s", resp.Status)
	}
	var events []event
	sc := bufio.NewScanner(resp.Body)
	var data strings.Builder
	for sc.Scan() {
		line := sc.Text()
		if line == "" {
			var ev event
			if err := json.Unmarshal([]byte(data.String()), &ev); err != nil {
				t.Fatalf("bad SSE frame %q: %v", data.String(), err)
			}
			events = append(events, ev)
			data.Reset()
			continue
		}
		if strings.HasPrefix(line, "data: ") {
			data.WriteString(strings.TrimPrefix(line, "data: "))
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	return events
}

func itoa(i int) string { return strconv.Itoa(i) }

func TestRunStreamExit(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("sh not available")
	}
	m := newRunManager()
	id, err := m.startFromCmd("echo", `sh -c "echo hi; exit 3"`, []string{"sh", "-c", "echo hi; exit 3"}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	events := waitEvents(m, id, 3*time.Second)
	if len(events) < 2 {
		t.Fatalf("got %d events, want >= 2: %+v", len(events), events)
	}
	last := events[len(events)-1]
	if last.Type != "exit" || last.Code != 3 {
		t.Fatalf("last event = %+v, want exit code 3", last)
	}
	if !strings.Contains(events[0].Text, "hi") {
		t.Fatalf("missing output: %+v", events)
	}
}

func TestRunStreamAndCancel(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("sh not available")
	}
	m := newRunManager()
	dir := t.TempDir()
	id, err := m.startFromCmd("sleep", "sleep 30", []string{"sh", "-c", "sleep 30"}, dir)
	if err != nil {
		t.Fatal(err)
	}
	rs, ok := m.get(id)
	if !ok {
		t.Fatal("run not registered")
	}
	rs.cancel()
	events := waitEvents(m, id, 5*time.Second)
	last := events[len(events)-1]
	if last.Type != "exit" {
		t.Fatalf("last event = %+v, want exit", last)
	}
	if last.Code == 0 {
		t.Fatalf("cancel should not exit 0, got %+v", last)
	}
}

func waitEvents(m *runManager, id int, timeout time.Duration) []event {
	rs, _ := m.get(id)
	_, wake, unsub := rs.subscribe()
	defer unsub()
	deadline := time.After(timeout)
	for {
		events, _, closed := rs.snapshot()
		if closed && len(events) > 0 {
			return events
		}
		select {
		case <-deadline:
			return events
		case <-wake:
		}
	}
}

func TestHandleLaunchValidation(t *testing.T) {
	s, ts := newTestServer(t, Options{})
	_ = s
	// No engine root: launcher.Command must fail with a friendly error.
	resp, err := http.Post(ts.URL+"/api/game/launch", "application/json", strings.NewReader(`{"wave":"2"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %s", resp.Status)
	}
	var out map[string]string
	json.NewDecoder(resp.Body).Decode(&out)
	if !strings.Contains(out["error"], "engine") && !strings.Contains(out["error"], "main.lua") {
		t.Fatalf("unexpected error: %v", out)
	}
}

func TestRunRequiresTask(t *testing.T) {
	_, ts := newTestServer(t, Options{})
	resp, err := http.Post(ts.URL+"/api/runs", "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %s", resp.Status)
	}
}

func TestTasksWhenJustMissing(t *testing.T) {
	_, ts := newTestServer(t, Options{JustPath: "", Justfile: filepath.Join(t.TempDir(), "justfile"), ModRoot: t.TempDir()})
	resp, err := http.Get(ts.URL + "/api/tasks")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out struct {
		Source string `json:"source"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Source != "builtin" {
		t.Fatalf("Source = %q, want builtin", out.Source)
	}
}
