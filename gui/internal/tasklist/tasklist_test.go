package tasklist

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// dumpFixture mirrors the shape of `just --dump --dump-format json` (1.58).
const dumpFixture = `{"recipes":{` +
	`"default":{"attributes":[],"body":[],"dependencies":[{"arguments":[],"recipe":"run","star":null}],"doc":null,"name":"default","parameters":[],"priors":1,"private":false,"quiet":false,"shebang":false},` +
	`"run":{"attributes":[],"body":[],"dependencies":[],"doc":null,"name":"run","parameters":[{"default":null,"export":false,"flag":false,"help":null,"kind":"star","long":null,"max":null,"min":null,"multiple":false,"name":"args","pattern":null,"short":null,"value":null}],"priors":0,"private":false,"quiet":false,"shebang":false},` +
	`"test":{"attributes":[],"body":[],"dependencies":[],"doc":"Run the smoke test","name":"test","parameters":[{"default":"x","export":false,"flag":true,"help":null,"kind":"flag","long":null,"max":null,"min":null,"multiple":false,"name":"force","pattern":null,"short":null,"value":null}],"priors":0,"private":false,"quiet":false,"shebang":false},` +
	`"secret":{"attributes":[],"body":[],"dependencies":[],"doc":"","name":"secret","parameters":[],"priors":0,"private":true,"quiet":false,"shebang":false}` +
	`},"aliases":{"l":{"attributes":[],"name":"l","target":"run"}}}`

const listFixture = `Available recipes:
    default
    run *args         # [alias: l]
    build (target)    # Build a thing
    test
    secret
`

func find(t *testing.T, tasks []Task, name string) *Task {
	t.Helper()
	for i := range tasks {
		if tasks[i].Name == name {
			return &tasks[i]
		}
	}
	t.Fatalf("task %q not found in %v", name, tasks)
	return nil
}

func TestParseDumpJSON(t *testing.T) {
	l, err := ParseDumpJSON([]byte(dumpFixture))
	if err != nil {
		t.Fatal(err)
	}
	if l.Source != "dump" {
		t.Fatalf("Source = %q, want dump", l.Source)
	}
	if len(l.Tasks) != 4 {
		t.Fatalf("got %d tasks, want 4", len(l.Tasks))
	}
	// Sorted by name.
	if l.Tasks[0].Name != "default" || l.Tasks[1].Name != "run" {
		t.Fatalf("tasks not sorted: %v", l.Tasks)
	}
	run := find(t, l.Tasks, "run")
	if len(run.Params) != 1 || run.Params[0].Kind != "star" || run.Params[0].Name != "args" {
		t.Fatalf("run params = %+v, want star args", run.Params)
	}
	if len(run.Aliases) != 1 || run.Aliases[0] != "l" {
		t.Fatalf("run aliases = %v, want [l]", run.Aliases)
	}
	test := find(t, l.Tasks, "test")
	if test.Doc != "Run the smoke test" {
		t.Fatalf("test doc = %q", test.Doc)
	}
	if len(test.Params) != 1 || !test.Params[0].Flag || test.Params[0].Default != "x" {
		t.Fatalf("test params = %+v, want flag param with default", test.Params)
	}
	if !find(t, l.Tasks, "secret").Private {
		t.Fatal("secret should be private")
	}
}

func TestParseDumpJSONGarbage(t *testing.T) {
	if _, err := ParseDumpJSON([]byte(`{"nope":1}`)); err == nil {
		t.Fatal("expected error for garbage dump")
	}
	if _, err := ParseDumpJSON([]byte(`not json`)); err == nil {
		t.Fatal("expected error for invalid json")
	}
}

func TestParseList(t *testing.T) {
	l := ParseList([]byte(listFixture))
	if l.Source != "list" {
		t.Fatalf("Source = %q, want list", l.Source)
	}
	if len(l.Tasks) != 5 {
		t.Fatalf("got %d tasks, want 5", len(l.Tasks))
	}
	run := find(t, l.Tasks, "run")
	if len(run.Params) != 1 || run.Params[0].Kind != "star" || run.Params[0].Name != "args" {
		t.Fatalf("run params = %+v", run.Params)
	}
	if len(run.Aliases) != 1 || run.Aliases[0] != "l" {
		t.Fatalf("run aliases = %v", run.Aliases)
	}
	build := find(t, l.Tasks, "build")
	if build.Doc != "Build a thing" {
		t.Fatalf("build doc = %q", build.Doc)
	}
	if len(build.Params) != 1 || build.Params[0].Name != "target" {
		t.Fatalf("build params = %+v", build.Params)
	}
	if find(t, l.Tasks, "default").Doc != "" {
		t.Fatal("default should have no doc")
	}
}

func TestParseListEmpty(t *testing.T) {
	l := ParseList([]byte(""))
	if len(l.Tasks) != 0 {
		t.Fatalf("got %v", l.Tasks)
	}
	l = ParseList([]byte("Available recipes:\n"))
	if len(l.Tasks) != 0 {
		t.Fatalf("got %v", l.Tasks)
	}
}

// TestLoad exercises the full path against the real just binary when
// available (skipped otherwise). It uses the repository's own justfile.
func TestLoad(t *testing.T) {
	if _, err := exec.LookPath("just"); err != nil {
		t.Skip("just not installed")
	}
	justPath, _ := exec.LookPath("just")
	// Repo root: this package's parent's parent's parent (gui/..).
	repoRoot := filepath.Clean(filepath.Join("..", "..", ".."))
	justfile, err := filepath.Abs(filepath.Join(repoRoot, "justfile"))
	if err != nil {
		t.Fatal(err)
	}
	if !fileExists(t, justfile) {
		t.Fatalf("justfile not found at %s", justfile)
	}
	l := Load(justPath, justfile, t.TempDir())
	if l.Source != "dump" {
		t.Fatalf("Source = %q, want dump (note: %s)", l.Source, l.Note)
	}
	find(t, l.Tasks, "run")
	find(t, l.Tasks, "test")
}

func TestLoadMissingJust(t *testing.T) {
	l := Load("/nonexistent/just", "/nonexistent/justfile", t.TempDir())
	if l.Source != "builtin" || l.Note == "" {
		t.Fatalf("got %+v, want builtin with note", l)
	}
}

func fileExists(t *testing.T, path string) bool {
	t.Helper()
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}
