// Package termrun opens the game and just tasks in a NEW terminal window,
// detached from the GUI process, so they get an interactive stdin/stdout
// (the terminal-cli console needs a real tty; piping /dev/null broke it).
package termrun

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/Bli-AIk/kristal-debug-tools/gui/internal/launcher"
)

// Command builds the terminal-emulator invocation that runs argv in a new
// interactive window. With pause, the window stays open after the command
// exits (for reading task output); without it, the window closes together
// with the command — used for the game, so closing the game closes its
// companion terminal. It is pure (no side effects) for testability.
func Command(title string, argv []string, dir string, pause bool) (*exec.Cmd, error) {
	cmdline := strings.Join(quote(argv), " ")
	switch runtime.GOOS {
	case "windows":
		// start "title" cmd <flag> <cmdline>: a new console window; /k keeps
		// it open after the command, /c closes it with the command.
		flag := "/k"
		if !pause {
			flag = "/c"
		}
		return exec.Command("cmd", "/c", "start", title, "cmd", flag, cmdline), nil
	default:
		term, err := findTerminal()
		if err != nil {
			return nil, err
		}
		wrapper := cmdline
		if pause {
			// Stay open for reading the output.
			wrapper += `; echo; echo "[kristal-debug-tools] finished — press Enter to close"; read _`
		}
		args := []string{"sh", "-c", wrapper}
		if strings.Contains(term, "gnome-terminal") {
			args = append([]string{"--"}, args...)
		}
		c := exec.Command(term, append([]string{"-e"}, args...)...)
		c.Dir = dir
		return c, nil
	}
}

// Spawn opens the terminal window and detaches from the GUI process.
func Spawn(title string, argv []string, dir string, pause bool) error {
	c, err := Command(title, argv, dir, pause)
	if err != nil {
		return err
	}
	if err := c.Start(); err != nil {
		return err
	}
	// Detach: the terminal owns its children; nothing to Wait on.
	c.Process.Release()
	return nil
}

var terminals = []string{
	"kitty", "gnome-terminal", "konsole", "xfce4-terminal",
	"x-terminal-emulator", "xterm",
}

func findTerminal() (string, error) {
	if t := os.Getenv("TERMINAL"); t != "" {
		if p, err := exec.LookPath(t); err == nil {
			return p, nil
		}
	}
	for _, t := range terminals {
		if p, err := exec.LookPath(t); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("no terminal emulator found (tried %s)", strings.Join(terminals, ", "))
}

func quote(argv []string) []string {
	out := make([]string, len(argv))
	for i, a := range argv {
		out[i] = launcher.ShellQuote(a)
	}
	return out
}
