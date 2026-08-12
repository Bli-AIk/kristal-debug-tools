package discover

import (
	"os"
	"path/filepath"
	"testing"
)

// TestModRoot exercises launcher resolution through the GUI entry. The repo
// layout (library inside a mod) makes walking up from the test cwd find the
// thrash-machine mod — that is the real behavior to lock in.
func TestModRoot(t *testing.T) {
	modRoot, engineRoot, modID := ModRoot("")
	if modRoot == "" {
		t.Fatal("expected mod discovery from repo layout")
	}
	if filepath.Base(modRoot) != "thrash-machine" {
		t.Fatalf("modRoot = %q, want thrash-machine", modRoot)
	}
	if modID != "thrash-machine" {
		t.Fatalf("modID = %q, want thrash-machine", modID)
	}
	if engineRoot == "" {
		t.Fatal("expected engine discovery (KRISTAL_ROOT or walk-up)")
	}
	// Explicit override that doesn't exist: degrade gracefully.
	if modRoot, _, _ = ModRoot("/nonexistent/root"); modRoot != "" {
		t.Fatalf("expected empty, got %q", modRoot)
	}
}

func TestJustfile(t *testing.T) {
	root := t.TempDir()

	t.Run("explicit path wins", func(t *testing.T) {
		p := filepath.Join(root, "custom.justfile")
		mustWrite(t, p)
		if got := Justfile(p, root); got != p {
			t.Fatalf("got %q, want %q", got, p)
		}
	})

	t.Run("mod root library layout", func(t *testing.T) {
		jf := filepath.Join(root, "libraries", "kristal-debug-tools", "justfile")
		mustWrite(t, jf)
		if got := Justfile("", root); got != jf {
			t.Fatalf("got %q, want %q", got, jf)
		}
	})

	t.Run("walk-up (dev layout)", func(t *testing.T) {
		// A build dropped into <repo>/dist finds <repo>/justfile by walking
		// up; os.Executable() points at the build cache in tests, so test
		// the walk-up helper directly.
		top := t.TempDir()
		jf := filepath.Join(top, "justfile")
		mustWrite(t, jf)
		deep := filepath.Join(top, "a", "b", "c")
		if err := os.MkdirAll(deep, 0o755); err != nil {
			t.Fatal(err)
		}
		if got := walkUpJustfile(deep); got != jf {
			t.Fatalf("got %q, want %q", got, jf)
		}
		// Walking past the filesystem root stops cleanly.
		if got := walkUpJustfile(filepath.Dir(top)); got == jf {
			t.Fatalf("unexpected %q above the fixture", got)
		}
	})
}

func TestEnvOr(t *testing.T) {
	if got := EnvOr("flag", "KDT_NOPE"); got != "flag" {
		t.Fatalf("got %q", got)
	}
	os.Setenv("KDT_TEST_ENVOR", "env")
	defer os.Unsetenv("KDT_TEST_ENVOR")
	if got := EnvOr("", "KDT_TEST_ENVOR"); got != "env" {
		t.Fatalf("got %q", got)
	}
}

func mustWrite(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
}
