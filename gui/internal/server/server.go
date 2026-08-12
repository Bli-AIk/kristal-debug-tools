// Package server hosts the GUI's HTTP API and embedded frontend. It binds to
// 127.0.0.1 only. Games and just tasks are launched in a NEW terminal window
// (internal/termrun) detached from the GUI; there are no arbitrary-command
// endpoints — runs are just recipes of a fixed justfile, and the game launch
// uses the launcher's whitelisted flags plus passthrough, exactly like the
// CLI.
package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"github.com/Bli-AIk/kristal-debug-tools/gui/internal/justbin"
	"github.com/Bli-AIk/kristal-debug-tools/gui/internal/launcher"
	"github.com/Bli-AIk/kristal-debug-tools/gui/internal/tasklist"
	"github.com/Bli-AIk/kristal-debug-tools/gui/internal/termrun"
	"github.com/Bli-AIk/kristal-debug-tools/gui/internal/web"
)

// bundledJustVersion must match JUST_VERSION in the justfile.
const bundledJustVersion = "1.58.0"

// Options is the static context the server is created with.
type Options struct {
	ModRoot     string // canonical mod root; working dir for just
	ModID       string
	EngineRoot  string
	Justfile    string // absolute path to the library justfile
	JustPath    string // resolved just executable ("" = none)
	JustMode    string // "system" | "embedded" | "cache" | "none"
	JustVersion string // best-effort "1.58.0"
	LovePath    string // "" = love not found

	// Spawn opens argv in a new interactive terminal window; pause keeps the
	// window open after the command exits. Overridable in tests.
	Spawn func(title string, argv []string, dir string, pause bool) error
}

// Server serves the GUI.
type Server struct {
	opts Options
	mux  *http.ServeMux
	runs *runManager
}

func New(opts Options) *Server {
	if opts.Spawn == nil {
		opts.Spawn = termrun.Spawn
	}
	s := &Server{opts: opts, runs: newRunManager()}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/status", s.handleStatus)
	mux.HandleFunc("GET /api/tasks", s.handleTasks)
	mux.HandleFunc("POST /api/tasks/refresh", s.handleTasks)
	mux.HandleFunc("POST /api/runs", s.handleRun)
	mux.HandleFunc("GET /api/runs", s.handleRuns)
	mux.HandleFunc("POST /api/game/launch", s.handleLaunch)
	mux.Handle("/", http.FileServer(http.FS(web.FS)))
	s.mux = mux
	return s
}

func (s *Server) Handler() http.Handler { return s.mux }

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

// --- endpoints ---

// libraryInfo is one entry of the project's libraries/ directory.
type libraryInfo struct {
	ID      string   `json:"id"`
	Version string   `json:"version"`
	Authors []string `json:"authors,omitempty"`
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	name, subtitle, libs := projectInfo(s.opts.ModRoot)
	engineVersion, engineHash := engineInfo(s.opts.EngineRoot)
	writeJSON(w, http.StatusOK, map[string]any{
		"modRoot":    s.opts.ModRoot,
		"modID":      s.opts.ModID,
		"engineRoot": s.opts.EngineRoot,
		"justfile":   s.opts.Justfile,
		"just": map[string]any{
			"mode":    s.opts.JustMode,
			"path":    s.opts.JustPath,
			"version": s.opts.JustVersion,
		},
		"love": map[string]any{
			"found": s.opts.LovePath != "",
			"path":  s.opts.LovePath,
		},
		"engine": map[string]any{
			"version": engineVersion,
			"hash":    engineHash,
		},
		"os":   runtime.GOOS,
		"arch": runtime.GOARCH,
		"project": map[string]any{
			"id":       s.opts.ModID,
			"name":     name,
			"subtitle": subtitle,
		},
		"libraries": libs,
	})
}

// engineInfo reads the engine's version (VERSION file at the repo root) and
// its git commit hash (best effort: .git/HEAD + loose refs). Both are empty
// strings when unavailable, e.g. a release ZIP without VERSION.
func engineInfo(engineRoot string) (version, hash string) {
	if engineRoot == "" {
		return "", ""
	}
	if data, err := os.ReadFile(filepath.Join(engineRoot, "VERSION")); err == nil {
		version = strings.TrimSpace(string(data))
	}
	head, err := os.ReadFile(filepath.Join(engineRoot, ".git", "HEAD"))
	if err != nil {
		return version, ""
	}
	s := strings.TrimSpace(string(head))
	if strings.HasPrefix(s, "ref: ") {
		if data, err := os.ReadFile(filepath.Join(engineRoot, ".git", strings.TrimPrefix(s, "ref: "))); err == nil {
			s = strings.TrimSpace(string(data))
		} else {
			return version, ""
		}
	}
	if len(s) >= 7 {
		hash = s[:7]
	}
	return version, hash
}

// projectInfo reads the mod's identity (mod.json) and its dependency
// libraries (libraries/*/lib.json). Kristal's JSON files are JSONC, so line
// comments are stripped before parsing; anything unreadable is skipped.
func projectInfo(modRoot string) (name, subtitle string, libs []libraryInfo) {
	if data, err := os.ReadFile(filepath.Join(modRoot, "mod.json")); err == nil {
		var m struct {
			Name     string `json:"name"`
			Subtitle string `json:"subtitle"`
		}
		if json.Unmarshal(stripJSONComments(data), &m) == nil {
			name, subtitle = m.Name, m.Subtitle
		}
	}
	entries, err := os.ReadDir(filepath.Join(modRoot, "libraries"))
	if err != nil {
		return name, subtitle, nil
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(modRoot, "libraries", e.Name(), "lib.json"))
		if err != nil {
			continue
		}
		var li libraryInfo
		if json.Unmarshal(stripJSONComments(data), &li) != nil || li.ID == "" {
			continue
		}
		libs = append(libs, li)
	}
	sort.Slice(libs, func(i, j int) bool { return libs[i].ID < libs[j].ID })
	return name, subtitle, libs
}

// stripJSONComments removes // line comments, /* */ blocks and trailing
// commas, keeping strings intact (a pragmatic JSONC preprocessor for
// mod.json / lib.json, which are written with comments and trailing commas).
func stripJSONComments(data []byte) []byte {
	var out bytes.Buffer
	inStr, esc := false, false
	trimTrailingComma := func() {
		// Drop a ',' plus following whitespace at the end of the buffer.
		b := out.Bytes()
		i := len(b) - 1
		for i >= 0 && (b[i] == ' ' || b[i] == '\t' || b[i] == '\n' || b[i] == '\r') {
			i--
		}
		if i >= 0 && b[i] == ',' {
			out.Truncate(i)
		}
	}
	for i := 0; i < len(data); i++ {
		c := data[i]
		if inStr {
			out.WriteByte(c)
			if esc {
				esc = false
			} else if c == '\\' {
				esc = true
			} else if c == '"' {
				inStr = false
			}
			continue
		}
		switch {
		case c == '"':
			inStr = true
			out.WriteByte(c)
		case c == '/' && i+1 < len(data) && data[i+1] == '/':
			for i < len(data) && data[i] != '\n' {
				i++
			}
			out.WriteByte('\n')
		case c == '/' && i+1 < len(data) && data[i+1] == '*':
			i += 2
			for i+1 < len(data) && !(data[i] == '*' && data[i+1] == '/') {
				i++
			}
			i++ // skip the closing '/'
		case c == '}' || c == ']':
			trimTrailingComma()
			out.WriteByte(c)
		default:
			out.WriteByte(c)
		}
	}
	return out.Bytes()
}

func (s *Server) handleTasks(w http.ResponseWriter, r *http.Request) {
	l := tasklist.Load(s.opts.JustPath, s.opts.Justfile, s.opts.ModRoot)
	writeJSON(w, http.StatusOK, l)
}

type runRequest struct {
	Task string   `json:"task"`
	Args []string `json:"args"`
}

func (s *Server) handleRun(w http.ResponseWriter, r *http.Request) {
	var req runRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad request: "+err.Error())
		return
	}
	if req.Task == "" {
		writeError(w, http.StatusBadRequest, "task is required")
		return
	}
	if s.opts.JustPath == "" {
		writeError(w, http.StatusBadRequest, "just is unavailable (see /api/status)")
		return
	}
	argv := []string{s.opts.JustPath, "--justfile", s.opts.Justfile, req.Task}
	argv = append(argv, req.Args...)
	cmdStr := strings.Join(quoteAll(argv), " ")
	if err := s.opts.Spawn(req.Task, argv, s.opts.ModRoot, true); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"id": s.runs.add(req.Task, cmdStr)})
}

func (s *Server) handleRuns(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"runs": s.runs.logSnapshot()})
}

// launchRequest mirrors the CLI launcher's flag surface (see
// bin/kristal-run): wave/encounter are free text (numeric or id selectors);
// tp/mercy accept either a number or a string.
type launchRequest struct {
	Lang        string   `json:"lang,omitempty"`
	Encounter   string   `json:"encounter,omitempty"`
	Wave        string   `json:"wave,omitempty"`
	WaveForce   string   `json:"waveForce,omitempty"`
	TP          any      `json:"tp,omitempty"`
	Mercy       any      `json:"mercy,omitempty"`
	Passthrough []string `json:"passthrough,omitempty"`
}

// stringify turns JSON numbers/strings/bools into CLI arg text.
func stringify(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(t)
	}
	return ""
}

func (s *Server) handleLaunch(w http.ResponseWriter, r *http.Request) {
	var req launchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad request: "+err.Error())
		return
	}
	// Build the CLI argv and run it through the launcher's own parser so the
	// GUI obeys exactly the same semantics as the command line (e.g. a bare
	// --encounter keeps the config default encounter).
	argv := []string{}
	if req.Lang != "" {
		argv = append(argv, "--lang", req.Lang)
	}
	if req.Encounter != "" {
		argv = append(argv, "--encounter", req.Encounter)
	}
	if req.Wave != "" {
		argv = append(argv, "--wave", req.Wave)
	}
	if req.WaveForce != "" {
		argv = append(argv, "--wave-force", req.WaveForce)
	}
	if tp := stringify(req.TP); tp != "" {
		argv = append(argv, "--tp", tp)
	}
	if mercy := stringify(req.Mercy); mercy != "" {
		argv = append(argv, "--mercy", mercy)
	}
	// Extra args are game args, i.e. what the CLI takes after `--`; the
	// launcher's own parser would reject unknown -* options otherwise.
	if len(req.Passthrough) > 0 {
		argv = append(argv, "--")
		argv = append(argv, req.Passthrough...)
	}

	args, err := launcher.ParseArgs(argv)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	cmd, err := launcher.Command(s.opts.EngineRoot, s.opts.ModID, args)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	cmdStr := strings.Join(quoteAll(cmd.Args), " ")
	title := "Kristal Debug — " + s.opts.ModID
	// The game's terminal closes together with the game window (no pause).
	if err := s.opts.Spawn(title, cmd.Args, cmd.Dir, false); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"id": s.runs.add("launch game", cmdStr)})
}

func quoteAll(argv []string) []string {
	out := make([]string, len(argv))
	for i, a := range argv {
		out[i] = launcher.ShellQuote(a)
	}
	return out
}

// --- just resolution ---

// ResolveJust finds a usable just: system PATH first (respects the user's
// environment), then the embedded copy, then the just.cmd cache.
func ResolveJust(justfileDir string) (path, mode, version string) {
	if p, err := exec.LookPath("just"); err == nil {
		if v := justVersion(p); v != "" {
			return p, "system", v
		}
	}
	if len(justbin.JustExe) > 0 {
		p, err := extractEmbeddedJust()
		if err == nil {
			return p, "embedded", bundledJustVersion
		}
	}
	if p := filepath.Join(justfileDir, ".tools", "just", "just.exe"); fileExists(p) {
		return p, "cache", ""
	}
	return "", "none", ""
}

// extractEmbeddedJust writes the embedded binary into the per-user cache
// directory, re-extracting only when the bytes differ (avoids AV churn).
func extractEmbeddedJust() (string, error) {
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(cache, "kristal-debug-tools")
	p := filepath.Join(dir, "just.exe")
	if cur, err := os.ReadFile(p); err == nil && bytes.Equal(cur, justbin.JustExe) {
		return p, nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(p, justbin.JustExe, 0o755); err != nil {
		return "", err
	}
	return p, nil
}

func justVersion(path string) string {
	out, err := exec.Command(path, "--version").Output()
	if err != nil {
		return ""
	}
	fields := strings.Fields(string(out))
	if len(fields) >= 2 {
		return fields[1]
	}
	return ""
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
