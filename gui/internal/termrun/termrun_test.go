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
	c, err := Command("title", []string{"love", "/path", "--mod", "m"}, "/dir")
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

func TestCommandWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows path")
	}
	c, err := Command("My Title", []string{"love", `C:\path`, "--mod", "m"}, `C:\dir`)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(c.Args, " ")
	for _, want := range []string{"cmd", "/c", "start", "My Title", "cmd", "/k"} {
		if !strings.Contains(got, want) {
			t.Fatalf("args %q missing %q", got, want)
		}
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
	if _, err := Command("t", []string{"x"}, ""); err == nil {
		t.Fatal("expected error when no terminal emulator exists")
	}
}
