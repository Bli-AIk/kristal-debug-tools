// Package launcher is a Go port of bin/kristal-run (the bash launcher).
//
// It must stay behavior-identical to the original: same flag surface, same
// error messages, same exit codes, same resolution precedence and the same
// KRISTAL_DEBUG_TOOLS_DRY_RUN output, so the port can be verified against the
// bash script byte for byte. bin/kristal-run is the source of truth; when in
// doubt, mirror it.
package launcher

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// usageText is byte-identical to bin/kristal-run's usage().
const usageText = `usage: kristal-run [--lang language|--language language|-l language] [--encounter [id]|-e [id]] [--wave selector|-w selector] [--wave-force selector|-wf selector]
                   [--tp n|--initial-tp n|-tp n] [--mercy n|--initial-mercy n|-m n]

  --lang, --language, -l
                         Select the startup language (for example, en or zh-hans).
  --encounter, -e       Start directly in an encounter.
  --wave, -w            Select a wave for the first defending phase.
  --wave-force, -wf     Select the same wave for every defending phase.
  --tp, --initial-tp    Set the starting battle TP. Use -tp as shorthand.
  --mercy, --initial-mercy, -m
                         Set the starting enemy mercy from 0 to 100.
  --                    Stop parsing launcher options and pass the rest to Kristal.`

// Usage returns the help text (exit 0 when --help is passed).
func Usage() string { return usageText }

// ErrHelp is returned by ParseArgs when --help/-h was seen (exit 0).
var ErrHelp = errors.New("help requested")

// MissingValueError is a flag that required a value but had none (exit 64).
// Msg is the flag token as typed, e.g. "--language" (bash: "$1 requires a value.").
type MissingValueError struct{ Flag string }

func (e *MissingValueError) Error() string { return fmt.Sprintf("%s requires a value.", e.Flag) }

// UnknownOptionError is an unrecognized launcher option (exit 64, usage shown).
type UnknownOptionError struct{ Flag string }

func (e *UnknownOptionError) Error() string { return fmt.Sprintf("unknown launcher option: %s", e.Flag) }

// ParseArgs mirrors bin/kristal-run's argument loop (lines 62-174). It returns
// the kristal args to pass to the game; --help yields ErrHelp; usage errors
// yield MissingValueError / UnknownOptionError.
func ParseArgs(argv []string) ([]string, error) {
	var out []string
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		switch {
		case a == "--help" || a == "-h":
			return nil, ErrHelp
		case a == "--":
			out = append(out, argv[i+1:]...)
			return out, nil
		case strings.HasPrefix(a, "--encounter="):
			// An empty value is legal and passes the bare flag, so the game
			// falls back to its configured default encounter.
			v := strings.TrimPrefix(a, "--encounter=")
			if v != "" {
				out = append(out, "--encounter", v)
			} else {
				out = append(out, "--encounter")
			}
		case a == "--encounter" || a == "-e":
			// Optional value: only consumed when the next arg is not a flag.
			if i+1 < len(argv) && !strings.HasPrefix(argv[i+1], "-") {
				out = append(out, "--encounter", argv[i+1])
				i++
			} else {
				out = append(out, "--encounter")
			}
		case strings.HasPrefix(a, "-e") && len(a) > 2:
			out = append(out, "--encounter", a[2:])
		case strings.HasPrefix(a, "--lang=") || strings.HasPrefix(a, "--language="):
			v := a[strings.IndexByte(a, '=')+1:]
			if v == "" {
				return nil, &MissingValueError{Flag: "--lang"}
			}
			out = append(out, "--lang", v)
		case a == "--lang" || a == "--language" || a == "-l":
			if i+1 >= len(argv) {
				return nil, &MissingValueError{Flag: a}
			}
			out = append(out, "--lang", argv[i+1])
			i++
		case strings.HasPrefix(a, "-l") && len(a) > 2:
			v := a[2:]
			if v == "" {
				return nil, &MissingValueError{Flag: "-l"}
			}
			out = append(out, "--lang", v)
		// Note: -wf / -wfX never reach the -wf cases below — the -w prefix
		// case matches them first, exactly like the bash script where `-w?*`
		// shadows the unreachable `-wf?*` branch (verified empirically). The
		// long form --wave-force works. Mirrored deliberately: the port must
		// stay byte-identical to bin/kristal-run.
		case strings.HasPrefix(a, "--wave="):
			v := strings.TrimPrefix(a, "--wave=")
			if v == "" {
				return nil, &MissingValueError{Flag: "--wave"}
			}
			out = append(out, "--wave", v)
		case a == "--wave" || a == "-w":
			if i+1 >= len(argv) {
				return nil, &MissingValueError{Flag: a}
			}
			out = append(out, "--wave", argv[i+1])
			i++
		case strings.HasPrefix(a, "-w") && len(a) > 2:
			v := a[2:]
			if v == "" {
				return nil, &MissingValueError{Flag: "-w"}
			}
			out = append(out, "--wave", v)
		case strings.HasPrefix(a, "--wave-force="):
			v := strings.TrimPrefix(a, "--wave-force=")
			if v == "" {
				return nil, &MissingValueError{Flag: "--wave-force"}
			}
			out = append(out, "--wave-force", v)
		case a == "--wave-force" || a == "-wf":
			if i+1 >= len(argv) {
				return nil, &MissingValueError{Flag: a}
			}
			out = append(out, "--wave-force", argv[i+1])
			i++
		case strings.HasPrefix(a, "-wf") && len(a) > 3:
			v := a[3:]
			if v == "" {
				return nil, &MissingValueError{Flag: "-wf"}
			}
			out = append(out, "--wave-force", v)
		case strings.HasPrefix(a, "--initial-tp=") || strings.HasPrefix(a, "--tp="):
			v := a[strings.IndexByte(a, '=')+1:]
			if v == "" {
				return nil, &MissingValueError{Flag: "--tp"}
			}
			out = append(out, "--tp", v)
		case a == "--initial-tp" || a == "--tp" || a == "-tp":
			if i+1 >= len(argv) {
				return nil, &MissingValueError{Flag: a}
			}
			out = append(out, "--tp", argv[i+1])
			i++
		case strings.HasPrefix(a, "-tp") && len(a) > 3:
			v := a[3:]
			if v == "" {
				return nil, &MissingValueError{Flag: "-tp"}
			}
			out = append(out, "--tp", v)
		case strings.HasPrefix(a, "--initial-mercy=") || strings.HasPrefix(a, "--mercy="):
			v := a[strings.IndexByte(a, '=')+1:]
			if v == "" {
				return nil, &MissingValueError{Flag: "--mercy"}
			}
			out = append(out, "--mercy", v)
		case a == "--initial-mercy" || a == "--mercy" || a == "-m":
			if i+1 >= len(argv) {
				return nil, &MissingValueError{Flag: a}
			}
			out = append(out, "--mercy", argv[i+1])
			i++
		case strings.HasPrefix(a, "-m") && len(a) > 2:
			v := a[2:]
			if v == "" {
				return nil, &MissingValueError{Flag: "-m"}
			}
			out = append(out, "--mercy", v)
		case strings.HasPrefix(a, "-"):
			return nil, &UnknownOptionError{Flag: a}
		default:
			out = append(out, a)
		}
	}
	return out, nil
}

// Resolved holds the paths computed by Resolve.
type Resolved struct {
	ModRoot    string // canonical (symlinks resolved)
	ModID      string
	EngineRoot string
}

// Resolve mirrors bin/kristal-run's resolution (lines 20-60 and 176-201).
//
// Precedence: KRISTAL_MOD_ROOT wins over walking up from cwd; the engine is
// found by walking up from the mod root (a project-local checkout stays
// authoritative), falling back to KRISTAL_ROOT. Empty env values are treated
// as unset, like bash's -z check. Exit codes: mod-root failure 1, engine
// failure 1 (mirroring the bash script; the caller maps errors to codes).
func Resolve(cwd, modRootEnv, kristalRootEnv string) (*Resolved, error) {
	// bash: find_mod_root starts from `pwd -P`; EvalSymlinks is the Go
	// equivalent of resolving the physical path.
	cwd, err := filepath.EvalSymlinks(cwd)
	if err != nil {
		return nil, fmt.Errorf("Could not resolve current directory: %v", err)
	}

	modRoot := modRootEnv
	if modRoot == "" {
		found, err := findModRoot(cwd)
		if err != nil {
			return nil, err
		}
		modRoot = found
	}
	// bash: mod_root=$(CDPATH= cd -- "$mod_root" && pwd -P)
	if modRoot, err = filepath.EvalSymlinks(modRoot); err != nil {
		return nil, fmt.Errorf("Could not resolve mod root: %v", err)
	}

	// Walk up from the mod root first; only then consider KRISTAL_ROOT.
	// (bash lines 176-201.)
	engineRoot := findEngine(modRoot)
	if engineRoot == "" {
		engineRoot = kristalRootEnv
	}
	if engineRoot == "" {
		return nil, errors.New("Kristal engine not found. Set KRISTAL_ROOT=/path/to/Kristal.")
	}

	return &Resolved{
		ModRoot:    modRoot,
		ModID:      modID(modRoot),
		EngineRoot: engineRoot,
	}, nil
}

// findModRoot walks up from dir looking for mod.json (bash lines 20-36).
func findModRoot(dir string) (string, error) {
	for {
		if fileExists(filepath.Join(dir, "mod.json")) {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", errors.New("Could not find mod.json. Run this command from a Kristal project or set KRISTAL_MOD_ROOT.")
}

// findEngine walks up from dir for a directory containing both main.lua and
// src/kristal.lua (bash lines 179-191).
func findEngine(dir string) string {
	for {
		if fileExists(filepath.Join(dir, "main.lua")) && fileExists(filepath.Join(dir, "src", "kristal.lua")) {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

// modID mirrors bash: first "id" string in mod.json, else the mod root's
// basename. json.Unmarshal is equivalent to the sed regex for well-formed
// JSON; on any parse failure we fall back to the basename.
func modID(modRoot string) string {
	if data, err := os.ReadFile(filepath.Join(modRoot, "mod.json")); err == nil {
		var m struct {
			ID string `json:"id"`
		}
		if json.Unmarshal(data, &m) == nil && m.ID != "" {
			return m.ID
		}
	}
	return filepath.Base(modRoot)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// LookupLove finds the love executable. PATH first (the bash script relies on
// exec finding love), then the usual Windows install locations.
func LookupLove() (string, error) {
	if p, err := exec.LookPath("love"); err == nil {
		return p, nil
	}
	if runtime.GOOS == "windows" {
		for _, dir := range []string{
			filepath.Join(os.Getenv("ProgramFiles"), "LOVE"),
			filepath.Join(os.Getenv("LOCALAPPDATA"), "Programs", "LOVE"),
		} {
			p := filepath.Join(dir, "love.exe")
			if fileExists(p) {
				return p, nil
			}
		}
	}
	return "", errors.New("love executable not found on PATH. Install LÖVE (https://love2d.org) or add its install directory to PATH.")
}

// Command builds the love invocation, mirroring bash line 219:
//
//	cd "$engine_root"; exec love "$engine_root" --mod "$mod_id" --auto-mod-start "${kristal_args[@]}"
//
// It also mirrors the pre-exec main.lua check (bash lines 213-216): the engine
// found via KRISTAL_ROOT may not actually be an engine tree.
func Command(engineRoot, modID string, args []string) (*exec.Cmd, error) {
	if !fileExists(filepath.Join(engineRoot, "main.lua")) {
		return nil, fmt.Errorf("Kristal engine main.lua not found: %s/main.lua", engineRoot)
	}
	love, err := LookupLove()
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(love, engineRoot, "--mod", modID, "--auto-mod-start")
	cmd.Args = append(cmd.Args, args...)
	cmd.Dir = engineRoot
	return cmd, nil
}

// ShellQuote quotes s like bash printf %q, keeping the dry-run output
// byte-identical to the bash script. Rules (verified against bash 5.x):
//   - safe characters (alphanumeric, _@%+=:,./- and !) stay literal
//   - other printable characters are backslash-escaped (space -> `\ `)
//   - control characters switch the whole string to ANSI-C form with octal
//     escapes ($'\001', $'\n', ...)
//   - the empty string becomes ''
func ShellQuote(s string) string {
	if s == "" {
		return "''"
	}
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			return ansiCQuote(s)
		}
	}
	var b strings.Builder
	for _, r := range s {
		if isShellSafe(r) {
			b.WriteRune(r)
		} else {
			b.WriteByte('\\')
			b.WriteRune(r)
		}
	}
	return b.String()
}

func isShellSafe(r rune) bool {
	return r > 0x7f || // bash leaves non-ASCII printable characters literal
		(r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') ||
		r == '_' || r == '@' || r == '%' || r == '+' || r == '=' || r == ':' ||
		r == ',' || r == '.' || r == '/' || r == '-' || r == '!'
}

// ansiCQuote renders s as bash's $'...' form (control chars in octal).
func ansiCQuote(s string) string {
	var b strings.Builder
	b.WriteString("$'")
	for _, r := range s {
		switch r {
		case '\n':
			b.WriteString(`\n`)
		case '\t':
			b.WriteString(`\t`)
		case '\r':
			b.WriteString(`\r`)
		case '\\':
			b.WriteString(`\\`)
		case '\'':
			b.WriteString(`\'`)
		default:
			if r < 0x20 || r == 0x7f {
				fmt.Fprintf(&b, `\%03o`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('\'')
	return b.String()
}

// DryRunString renders the KRISTAL_DEBUG_TOOLS_DRY_RUN output (bash lines
// 203-211): three label lines plus the shell-quoted love invocation. The
// first quoted arg is the engine root (the love executable itself is resolved
// by PATH at exec time, exactly like bash's `exec love ...`).
func DryRunString(res *Resolved, args []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "mod_root=%s\n", res.ModRoot)
	fmt.Fprintf(&b, "mod_id=%s\n", res.ModID)
	fmt.Fprintf(&b, "engine_root=%s\n", res.EngineRoot)
	b.WriteString("love")
	for _, a := range append([]string{res.EngineRoot, "--mod", res.ModID, "--auto-mod-start"}, args...) {
		b.WriteByte(' ')
		b.WriteString(ShellQuote(a))
	}
	b.WriteByte('\n')
	return b.String()
}
