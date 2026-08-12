package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func newTestServer(t *testing.T, opts Options) (*Server, *httptest.Server, *fakeSpawner) {
	t.Helper()
	spawn := &fakeSpawner{}
	opts.Spawn = spawn.Spawn
	s := New(opts)
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)
	return s, ts, spawn
}

// fakeSpawner records spawns instead of opening terminal windows.
type fakeSpawner struct {
	mu   sync.Mutex
	args [][]string
	dirs []string
}

func (f *fakeSpawner) Spawn(_ string, argv []string, dir string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.args = append(f.args, append([]string(nil), argv...))
	f.dirs = append(f.dirs, dir)
	return nil
}

func (f *fakeSpawner) last() ([]string, string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.args) == 0 {
		return nil, ""
	}
	return f.args[len(f.args)-1], f.dirs[len(f.dirs)-1]
}

func TestRunTaskSpawnsInTerminal(t *testing.T) {
	_, ts, spawn := newTestServer(t, Options{
		ModRoot: t.TempDir(), JustPath: "/usr/bin/just",
		Justfile: filepath.Join(t.TempDir(), "justfile"),
	})
	resp, err := http.Post(ts.URL+"/api/runs", "application/json", strings.NewReader(`{"task":"test","args":["--x"]}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %s", resp.Status)
	}
	var out struct {
		ID int `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	argv, dir := spawn.last()
	if argv == nil {
		t.Fatal("no spawn recorded")
	}
	// argv: just --justfile <path> test --x
	if argv[0] != "/usr/bin/just" || argv[3] != "test" || argv[4] != "--x" {
		t.Fatalf("unexpected argv: %v", argv)
	}
	// Runs log records the spawn.
	resp2, err := http.Get(ts.URL + "/api/runs")
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	var log struct {
		Runs []runEntry `json:"runs"`
	}
	if err := json.NewDecoder(resp2.Body).Decode(&log); err != nil {
		t.Fatal(err)
	}
	if len(log.Runs) != 1 || log.Runs[0].Label != "test" {
		t.Fatalf("unexpected log: %+v", log.Runs)
	}
	_ = dir
}

func TestRunRequiresTaskAndJust(t *testing.T) {
	_, ts, _ := newTestServer(t, Options{})
	resp, err := http.Post(ts.URL+"/api/runs", "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %s", resp.Status)
	}

	_, ts2, _ := newTestServer(t, Options{JustPath: ""})
	resp2, err := http.Post(ts2.URL+"/api/runs", "application/json", strings.NewReader(`{"task":"test"}`))
	if err != nil {
		t.Fatal(err)
	}
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %s", resp2.Status)
	}
}

func TestHandleLaunchValidation(t *testing.T) {
	_, ts, _ := newTestServer(t, Options{})
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

func TestHandleLaunchBuildsArgs(t *testing.T) {
	root := t.TempDir()
	engine := filepath.Join(root, "engine")
	mustWrite(t, filepath.Join(engine, "main.lua"), nil)
	_, ts, spawn := newTestServer(t, Options{
		EngineRoot: engine, ModID: "thrash-machine",
	})
	resp, err := http.Post(ts.URL+"/api/game/launch", "application/json",
		strings.NewReader(`{"wave":"2","tp":50,"mercy":"75","passthrough":["--foo"]}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %s (%v)", resp.Status, resp.Body)
	}
	argv, dir := spawn.last()
	if argv == nil {
		t.Fatal("no spawn recorded")
	}
	joined := strings.Join(argv, " ")
	for _, want := range []string{"--mod thrash-machine", "--wave 2", "--tp 50", "--mercy 75", "--foo"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("argv %q missing %q", joined, want)
		}
	}
	if dir != engine {
		t.Fatalf("dir = %q, want %q", dir, engine)
	}
}

func TestTasksWhenJustMissing(t *testing.T) {
	_, ts, _ := newTestServer(t, Options{JustPath: "", Justfile: filepath.Join(t.TempDir(), "justfile"), ModRoot: t.TempDir()})
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

func TestProjectInfoJSONC(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "mod.json"), []byte(`{
		// JSONC comments
		"id": "mod", "name": "My Mod", "subtitle": "a test",
	}`))
	mustWrite(t, filepath.Join(root, "libraries", "kristal-i18n", "lib.json"), []byte(`{
		"id": "kristal-i18n",
		"version": "v0.1.0", // comment
		"authors": ["a", "b"]
	}`))
	name, subtitle, libs := projectInfo(root)
	if name != "My Mod" || subtitle != "a test" {
		t.Fatalf("got %q %q", name, subtitle)
	}
	if len(libs) != 1 || libs[0].ID != "kristal-i18n" || libs[0].Version != "v0.1.0" {
		t.Fatalf("libs = %+v", libs)
	}
}

func mustWrite(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}
