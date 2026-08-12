package termrun

import (
	"runtime"
	"strings"
	"testing"
)

func TestCommandPosix(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("posix path")
	}
	t.Setenv("TERMINAL", "/bin/true") // deterministic terminal
	c, err := Command("title", []string{"love", "/path", "--mod", "m"}, "/dir", true)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(c.Args, " ")
	for _, want := range []string{"/bin/true", "-e", "sh", "-c", "love /path --mod m", "press Enter to close"} {
		if !strings.Contains(got, want) {
			t.Fatalf("args %q missing %q", got, want)
		}
	}
	if c.Dir != "/dir" {
		t.Fatalf("Dir = %q", c.Dir)
	}
}

func TestCommandPosixNoPause(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("posix path")
	}
	t.Setenv("TERMINAL", "/bin/true")
	c, err := Command("title", []string{"love", "/path"}, "/dir", false)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(c.Args, " ")
	if strings.Contains(got, "press Enter") {
		t.Fatalf("no-pause command must not keep the window open: %q", got)
	}
	if !strings.Contains(got, "love /path") {
		t.Fatalf("args %q missing the command", got)
	}
}

func TestCommandWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows path")
	}
	c, err := Command("My Title", []string{"love", `C:\path`, "--mod", "m"}, `C:\dir`, true)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(c.Args, " ")
	for _, want := range []string{"cmd", "/c", "start", "My Title", "cmd", "/k"} {
		if !strings.Contains(got, want) {
			t.Fatalf("args %q missing %q", got, want)
		}
	}
	// No pause -> /c so the window closes with the command.
	c2, err := Command("t", []string{"love"}, `C:\dir`, false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(c2.Args, " "), `cmd /c start t cmd /c love`) {
		t.Fatalf("no-pause args = %v", c2.Args)
	}
}

func TestNoTerminal(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("posix path")
	}
	t.Setenv("TERMINAL", "")
	// Point PATH at an empty dir so no terminal emulator is found.
	empty := t.TempDir()
	t.Setenv("PATH", empty)
	if _, err := Command("t", []string{"x"}, "", true); err == nil {
		t.Fatal("expected error when no terminal emulator exists")
	}
}
