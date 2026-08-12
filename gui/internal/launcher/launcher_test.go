package launcher

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func eq(t *testing.T, name string, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: got %v, want %v", name, got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%s: got %v, want %v", name, got, want)
		}
	}
}

// TestParseArgs mirrors tests/smoke.sh's positive cases plus attached and
// edge forms not covered there.
func TestParseArgs(t *testing.T) {
	cases := []struct {
		name string
		argv []string
		want []string
	}{
		{"wave tp mercy", []string{"--wave", "2", "--tp", "50", "--mercy", "100"},
			[]string{"--wave", "2", "--tp", "50", "--mercy", "100"}},
		{"encounter long forms", []string{"--encounter", "dummy", "--initial-tp=25", "--initial-mercy=75"},
			[]string{"--encounter", "dummy", "--tp", "25", "--mercy", "75"}},
		{"lang space", []string{"--lang", "zh-hans"}, []string{"--lang", "zh-hans"}},
		{"language equals", []string{"--language=en"}, []string{"--lang", "en"}},
		{"short lang", []string{"-l", "zh-hans"}, []string{"--lang", "zh-hans"}},
		{"double dash", []string{"--", "--custom", "value"}, []string{"--custom", "value"}},
		{"attached tp", []string{"-tp25"}, []string{"--tp", "25"}},
		{"attached encounter wave", []string{"-eDummy", "-w2", "-wf2", "-m50"},
			[]string{"--encounter", "Dummy", "--wave", "2", "--wave-force", "2", "--mercy", "50"}},
		{"short wave-force", []string{"-wf", "2"}, []string{"--wave-force", "2"}},
		{"long wave-force", []string{"--wave-force", "2"}, []string{"--wave-force", "2"}},
		{"bare encounter", []string{"--encounter"}, []string{"--encounter"}},
		{"empty encounter value", []string{"--encounter="}, []string{"--encounter"}},
		{"encounter eats nothing flag", []string{"--encounter", "--wave", "2"},
			[]string{"--encounter", "--wave", "2"}},
		{"positional passthrough", []string{"run", "--wave", "2"}, []string{"run", "--wave", "2"}},
		{"mixed forms", []string{"--tp=10", "--encounter=foo", "-l", "en", "extra"},
			[]string{"--tp", "10", "--encounter", "foo", "--lang", "en", "extra"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseArgs(tc.argv)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			eq(t, "args", got, tc.want)
		})
	}
}

// TestParseArgsErrors mirrors smoke.sh's validation cases: missing values and
// unknown options must fail with the exact messages (exit 64 upstream).
func TestParseArgsErrors(t *testing.T) {
	t.Run("missing tp", func(t *testing.T) {
		_, err := ParseArgs([]string{"--tp"})
		var mv *MissingValueError
		if !errors.As(err, &mv) || mv.Flag != "--tp" {
			t.Fatalf("got %v, want MissingValueError(--tp)", err)
		}
	})
	t.Run("missing language", func(t *testing.T) {
		_, err := ParseArgs([]string{"--language"})
		var mv *MissingValueError
		if !errors.As(err, &mv) || mv.Flag != "--language" {
			t.Fatalf("got %v, want MissingValueError(--language)", err)
		}
	})
	t.Run("unknown option", func(t *testing.T) {
		_, err := ParseArgs([]string{"--unknown"})
		var un *UnknownOptionError
		if !errors.As(err, &un) || un.Flag != "--unknown" {
			t.Fatalf("got %v, want UnknownOptionError(--unknown)", err)
		}
	})
	t.Run("empty lang value", func(t *testing.T) {
		_, err := ParseArgs([]string{"--lang="})
		var mv *MissingValueError
		if !errors.As(err, &mv) || mv.Flag != "--lang" {
			t.Fatalf("got %v, want MissingValueError(--lang)", err)
		}
	})
	t.Run("empty mercy value", func(t *testing.T) {
		_, err := ParseArgs([]string{"--mercy="})
		var mv *MissingValueError
		if !errors.As(err, &mv) || mv.Flag != "--mercy" {
			t.Fatalf("got %v, want MissingValueError(--mercy)", err)
		}
	})
	t.Run("short wave-force needs value", func(t *testing.T) {
		_, err := ParseArgs([]string{"-wf"})
		var mv *MissingValueError
		if !errors.As(err, &mv) || mv.Flag != "-wf" {
			t.Fatalf("got %v, want MissingValueError(-wf)", err)
		}
	})
	t.Run("help", func(t *testing.T) {
		if _, err := ParseArgs([]string{"-h"}); !errors.Is(err, ErrHelp) {
			t.Fatalf("got %v, want ErrHelp", err)
		}
	})
}

// TestResolve mirrors smoke.sh's engine-precedence fixture and adds the
// fallback cases.
func TestResolve(t *testing.T) {
	root := t.TempDir()
	engine := filepath.Join(root, "engine")
	mustMkdirAll(t, filepath.Join(engine, "src"))
	mustWriteFile(t, filepath.Join(engine, "main.lua"), nil)
	mustWriteFile(t, filepath.Join(engine, "src", "kristal.lua"), nil)
	mod := filepath.Join(engine, "mod")
	mustMkdirAll(t, mod)
	mustWriteFile(t, filepath.Join(mod, "mod.json"), []byte(`{"id": "test-mod"}`))

	t.Run("engine walk-up beats KRISTAL_ROOT", func(t *testing.T) {
		res, err := Resolve(mod, "", filepath.Join(root, "override"))
		if err != nil {
			t.Fatal(err)
		}
		if res.ModRoot != mod {
			t.Fatalf("ModRoot = %q, want %q", res.ModRoot, mod)
		}
		if res.ModID != "test-mod" {
			t.Fatalf("ModID = %q, want %q", res.ModID, "test-mod")
		}
		if res.EngineRoot != engine {
			t.Fatalf("EngineRoot = %q, want %q", res.EngineRoot, engine)
		}
	})

	t.Run("KRISTAL_MOD_ROOT wins", func(t *testing.T) {
		// cwd without mod.json; env var points at the mod.
		res, err := Resolve(root, mod, "")
		if err != nil {
			t.Fatal(err)
		}
		if res.ModRoot != mod {
			t.Fatalf("ModRoot = %q, want %q", res.ModRoot, mod)
		}
	})

	t.Run("mod id falls back to basename", func(t *testing.T) {
		plain := filepath.Join(root, "plain-mod")
		mustMkdirAll(t, plain)
		mustWriteFile(t, filepath.Join(plain, "mod.json"), []byte(`{}`))
		res, err := Resolve(plain, "", engine)
		if err != nil {
			t.Fatal(err)
		}
		if res.ModID != "plain-mod" {
			t.Fatalf("ModID = %q, want %q", res.ModID, "plain-mod")
		}
	})

	t.Run("KRISTAL_ROOT fallback", func(t *testing.T) {
		// Mod outside any engine tree; KRISTAL_ROOT provides the engine.
		outside := filepath.Join(root, "outside")
		mustMkdirAll(t, outside)
		mustWriteFile(t, filepath.Join(outside, "mod.json"), nil)
		res, err := Resolve(outside, "", engine)
		if err != nil {
			t.Fatal(err)
		}
		if res.EngineRoot != engine {
			t.Fatalf("EngineRoot = %q, want %q", res.EngineRoot, engine)
		}
	})

	t.Run("no mod root", func(t *testing.T) {
		_, err := Resolve(root, "", "")
		if err == nil || err.Error() != "Could not find mod.json. Run this command from a Kristal project or set KRISTAL_MOD_ROOT." {
			t.Fatalf("got %v", err)
		}
	})

	t.Run("no engine", func(t *testing.T) {
		plain := filepath.Join(root, "no-engine")
		mustMkdirAll(t, plain)
		mustWriteFile(t, filepath.Join(plain, "mod.json"), nil)
		_, err := Resolve(plain, "", "")
		if err == nil || err.Error() != "Kristal engine not found. Set KRISTAL_ROOT=/path/to/Kristal." {
			t.Fatalf("got %v", err)
		}
	})
}

func TestShellQuote(t *testing.T) {
	cases := []struct{ in, want string }{
		{"abc", "abc"},
		{"a b", `a\ b`},
		{"a'b", `a\'b`},
		{"a\"b", `a\"b`},
		{"a$b", `a\$b`},
		{"a;b", `a\;b`},
		{"x!", "x!"},
		{"a_b", "a_b"},
		{"中", "中"},
		{"", "''"},
		{"/path/to/engine", "/path/to/engine"},
		{"a\nb", `$'a\nb'`},
		{"a\tb", `$'a\tb'`},
		{"\x01", `$'\001'`},
	}
	for _, tc := range cases {
		if got := ShellQuote(tc.in); got != tc.want {
			t.Fatalf("ShellQuote(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestCommand(t *testing.T) {
	root := t.TempDir()
	engine := filepath.Join(root, "engine")
	mustMkdirAll(t, engine)
	mustWriteFile(t, filepath.Join(engine, "main.lua"), nil)

	t.Run("missing main.lua", func(t *testing.T) {
		_, err := Command(filepath.Join(root, "nope"), "mod", nil)
		if err == nil {
			t.Fatal("expected error for missing main.lua")
		}
	})

	t.Run("args and dir", func(t *testing.T) {
		if _, err := LookupLove(); err != nil {
			t.Skip("love not installed on this machine")
		}
		cmd, err := Command(engine, "mod", []string{"--wave", "2"})
		if err != nil {
			t.Fatal(err)
		}
		if cmd.Dir != engine {
			t.Fatalf("Dir = %q, want %q", cmd.Dir, engine)
		}
		want := []string{cmd.Args[0], engine, "--mod", "mod", "--auto-mod-start", "--wave", "2"}
		eq(t, "args", cmd.Args, want)
	})
}

func mustMkdirAll(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustWriteFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}
