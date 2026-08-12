package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
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

func (f *fakeSpawner) Spawn(_ string, argv []string, dir string, _ bool) error {
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

func TestProjectTasksAndRun(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "justfile"), []byte("build:\n\t@echo build\n"))
	_, ts, spawn := newTestServer(t, Options{
		ModRoot: root, JustPath: "/usr/bin/just",
		Justfile: filepath.Join(t.TempDir(), "lib-justfile"),
	})
	// Tasks include the project's recipes as a second group.
	resp, err := http.Get(ts.URL + "/api/tasks")
	if err != nil {
		t.Fatal(err)
	}
	var out struct {
		Tasks []struct{ Name string } `json:"tasks"`
		Mod   *struct {
			Tasks []struct{ Name string } `json:"tasks"`
		} `json:"mod"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if out.Mod == nil || len(out.Mod.Tasks) != 1 || out.Mod.Tasks[0].Name != "build" {
		t.Fatalf("mod group = %+v", out.Mod)
	}
	// Running a project task uses the project justfile.
	resp2, err := http.Post(ts.URL+"/api/runs", "application/json",
		strings.NewReader(`{"task":"build","justfile":"project"}`))
	if err != nil {
		t.Fatal(err)
	}
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %s", resp2.Status)
	}
	argv, _ := spawn.last()
	if argv == nil || !strings.HasSuffix(argv[2], "justfile") {
		t.Fatalf("argv = %v", argv)
	}
	if filepath.Base(argv[2]) != "justfile" || filepath.Dir(argv[2]) != root {
		t.Fatalf("project run used %q, want %q", argv[2], filepath.Join(root, "justfile"))
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

func TestEngineInfo(t *testing.T) {
	engine := t.TempDir()
	mustWrite(t, filepath.Join(engine, "VERSION"), []byte("v0.10.0\n"))
	mustWrite(t, filepath.Join(engine, ".git", "HEAD"), []byte("ref: refs/heads/main\n"))
	mustWrite(t, filepath.Join(engine, ".git", "refs", "heads", "main"), []byte("752bc0688ba97ca8a256ba9125b7e05a1ca6edbd\n"))
	version, hash := engineInfo(engine)
	if version != "v0.10.0" || hash != "752bc06" {
		t.Fatalf("got %q %q, want v0.10.0 752bc06", version, hash)
	}
	// Missing git metadata: version only.
	plain := t.TempDir()
	mustWrite(t, filepath.Join(plain, "VERSION"), []byte("v1.2.3"))
	if version, hash = engineInfo(plain); version != "v1.2.3" || hash != "" {
		t.Fatalf("got %q %q", version, hash)
	}
	// Empty root.
	if version, hash = engineInfo(""); version != "" || hash != "" {
		t.Fatalf("got %q %q", version, hash)
	}
}

func TestTemplateInfo(t *testing.T) {
	root := t.TempDir()
	engine := t.TempDir()
	mustWrite(t, filepath.Join(engine, "configs", "chapter1.json"), []byte("{\n  \"a\": 1, // c\n  \"b\": 2\n}"))
	mustWrite(t, filepath.Join(engine, "configs", "chapter2.json"), []byte(`{"x": true}`))
	// No start.sh -> not a template.
	if info := detectTemplate(root, engine); info != nil {
		t.Fatalf("expected nil without start.sh, got %+v", info)
	}
	mustWrite(t, filepath.Join(root, "start.sh"), []byte("#!/bin/sh\n"))
	// Wrong subtitle -> not a template.
	mustWrite(t, filepath.Join(root, "mod.json"), []byte(`{"subtitle": "other"}`))
	if info := detectTemplate(root, engine); info != nil {
		t.Fatalf("expected nil with wrong subtitle, got %+v", info)
	}
	// Real template.
	mustWrite(t, filepath.Join(root, "mod.json"), []byte(`{
		"name": "Thrash Machine",
		"subtitle": "Kristal Lua template",
		"chapter": 3,
	}`))
	info := detectTemplate(root, engine)
	if info == nil || !info.IsTemplate {
		t.Fatal("expected template detection")
	}
	if info.Chapter != 3 || info.Name != filepath.Base(root) {
		t.Fatalf("info = %+v", info)
	}
	if len(info.Chapters) != 2 || info.Chapters[0].Items != 2 {
		t.Fatalf("chapters = %+v", info.Chapters)
	}
}

// TestTemplateAlreadyInitialized: start.sh never rewrites the subtitle, so
// the marker alone would keep showing the panel after init; the git-HEAD
// id/name comparison must hide it.
func TestTemplateAlreadyInitialized(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	root := t.TempDir()
	engine := t.TempDir()
	mustWrite(t, filepath.Join(root, "start.sh"), []byte("#!/bin/sh\n"))
	modJSON := []byte(`{
		"id": "thrash-machine",
		"name": "Thrash Machine",
		"subtitle": "Kristal Lua template",
		"chapter": 4,
	}`)
	mustWrite(t, filepath.Join(root, "mod.json"), modJSON)
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q")
	run("add", "mod.json", "start.sh")
	run("commit", "-q", "-m", "template")
	if info := detectTemplate(root, engine); info == nil || !info.IsTemplate {
		t.Fatal("pristine template should be detected")
	}
	// Simulate initialization: id/name changed, subtitle untouched.
	mustWrite(t, filepath.Join(root, "mod.json"), []byte(`{
		"id": "my-game",
		"name": "My Game",
		"subtitle": "Kristal Lua template",
		"chapter": 2,
	}`))
	if info := detectTemplate(root, engine); info != nil {
		t.Fatalf("initialized project must hide the panel, got %+v", info)
	}
}

func TestTemplateChapterPreservesJSONC(t *testing.T) {
	root := t.TempDir()
	modJSON := `{
	// The Deltarune chapter you'd like to base your project off of.
	"chapter": 4,
	"name": "Thrash Machine", // display name
}`
	mustWrite(t, filepath.Join(root, "mod.json"), []byte(modJSON))
	_, ts, _ := newTestServer(t, Options{ModRoot: root})
	resp, err := http.Post(ts.URL+"/api/template/chapter", "application/json",
		strings.NewReader(`{"chapter":2}`))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %s", resp.Status)
	}
	out, err := os.ReadFile(filepath.Join(root, "mod.json"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, `"chapter": 2`) {
		t.Fatalf("chapter not updated: %s", s)
	}
	for _, keep := range []string{"The Deltarune chapter", "display name"} {
		if !strings.Contains(s, keep) {
			t.Fatalf("comment lost (%q): %s", keep, s)
		}
	}
	// Invalid chapter rejected.
	resp2, _ := http.Post(ts.URL+"/api/template/chapter", "application/json",
		strings.NewReader(`{"chapter":9}`))
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for chapter 9, got %s", resp2.Status)
	}
}

func TestTemplateInitValidation(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "mod.json"), []byte(`{"subtitle": "Kristal Lua template", "chapter": 1}`))
	mustWrite(t, filepath.Join(root, "start.sh"), []byte("#!/bin/sh\n"))
	_, ts, spawn := newTestServer(t, Options{ModRoot: root, EngineRoot: t.TempDir()})
	// Bad names rejected.
	for _, name := range []string{"", "a/b", "a\\b", "a:b", "a*b", "a<b"} {
		resp, err := http.Post(ts.URL+"/api/template/init", "application/json",
			strings.NewReader(`{"name":"`+name+`"}`))
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("name %q: expected 400, got %s", name, resp.Status)
		}
	}
	// Good name spawns bash start.sh.
	resp, err := http.Post(ts.URL+"/api/template/init", "application/json",
		strings.NewReader(`{"name":"My Cool Project"}`))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %s", resp.Status)
	}
	argv, _ := spawn.last()
	if argv == nil || argv[0] != "bash" || !strings.HasSuffix(argv[1], "start.sh") || argv[3] != "My Cool Project" {
		t.Fatalf("argv = %v", argv)
	}
}

func TestChapterConfig(t *testing.T) {
	engine := t.TempDir()
	mustWrite(t, filepath.Join(engine, "configs", "chapter1.json"), []byte(`{"oldTensionBar": true, "darkCurrency": "Dark Dollars"}`))
	mustWrite(t, filepath.Join(engine, "configs", "chapter2.json"), []byte(`{"oldTensionBar": false, "darkCurrency": "Dark Dollars"}`))
	mustWrite(t, filepath.Join(engine, "configs", "chapter3.json"), []byte(`{"oldTensionBar": false}`))
	mustWrite(t, filepath.Join(engine, "configs", "chapter4.json"), []byte(`{"oldTensionBar": false}`))
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "mod.json"), []byte(`{
		"chapter": 2,
		"config": {
			"kristal": {
				"oldTensionBar": true, // override
			},
		},
	}`))
	out := chapterConfig(root, engine)
	items := out["items"].([]chapterConfigItem)
	var tension *chapterConfigItem
	for i := range items {
		if items[i].Key == "oldTensionBar" {
			tension = &items[i]
		}
	}
	if tension == nil {
		t.Fatal("oldTensionBar not listed")
	}
	if tension.Values["1"] != true || tension.Values["2"] != false {
		t.Fatalf("values = %+v", tension.Values)
	}
	if tension.Override != true {
		t.Fatalf("override = %+v", tension.Override)
	}
	if tension.Desc == "" {
		t.Fatal("expected zh description from config-features.json")
	}
	if out["chapter"] != 2 {
		t.Fatalf("chapter = %v", out["chapter"])
	}
}

func TestModConfigSet(t *testing.T) {
	root := t.TempDir()
	mod := `{
	// chapter comment
	"chapter": 4,
	"config": {
		"kristal": {
			// kristal block comment
			"oldTensionBar": true, // inline comment
		},
		"kristalI18n": {},
	},
}`
	mustWrite(t, filepath.Join(root, "mod.json"), []byte(mod))

	// Update an existing key, preserving comments.
	if err := modConfigSet(root, "oldTensionBar", false); err != nil {
		t.Fatal(err)
	}
	out, _ := os.ReadFile(filepath.Join(root, "mod.json"))
	s := string(out)
	for _, keep := range []string{"chapter comment", "kristal block comment", "inline comment", `"chapter": 4`} {
		if !strings.Contains(s, keep) {
			t.Fatalf("lost %q: %s", keep, s)
		}
	}
	if !strings.Contains(s, `"oldTensionBar": false`) {
		t.Fatalf("not updated: %s", s)
	}

	// Insert a new key.
	if err := modConfigSet(root, "growStronger", true); err != nil {
		t.Fatal(err)
	}
	out, _ = os.ReadFile(filepath.Join(root, "mod.json"))
	if !strings.Contains(string(out), `"growStronger": true`) {
		t.Fatalf("not inserted: %s", out)
	}

	// Remove a key.
	if err := modConfigSet(root, "oldTensionBar", nil); err != nil {
		t.Fatal(err)
	}
	out, _ = os.ReadFile(filepath.Join(root, "mod.json"))
	if strings.Contains(string(out), "oldTensionBar") {
		t.Fatalf("not removed: %s", out)
	}
	if !strings.Contains(string(out), "growStronger") {
		t.Fatalf("other key lost: %s", out)
	}

	// Round-trip: the result still parses as JSONC.
	var m map[string]any
	if err := json.Unmarshal(stripJSONComments(out), &m); err != nil {
		t.Fatalf("result not parseable: %v\n%s", err, out)
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
