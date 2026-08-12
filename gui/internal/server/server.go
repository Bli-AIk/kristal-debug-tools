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
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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
	mux.HandleFunc("POST /api/template/init", s.handleTemplateInit)
	mux.HandleFunc("POST /api/template/chapter", s.handleTemplateChapter)
	mux.HandleFunc("GET /api/chapter-config", s.handleChapterConfig)
	mux.HandleFunc("POST /api/chapter-config", s.handleChapterConfigSet)
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
		"template":  detectTemplate(s.opts.ModRoot, s.opts.EngineRoot),
	})
}

// --- template initialization (thrash-machine) ---

type chapterInfo struct {
	Number int `json:"number"`
	Items  int `json:"items"`
}

type templateInfo struct {
	IsTemplate bool          `json:"isTemplate"`
	Name       string        `json:"name"`    // suggested project name (mod dir basename)
	Chapter    int           `json:"chapter"` // current value in mod.json
	Chapters   []chapterInfo `json:"chapters"`
}

// detectTemplate detects a PRISTINE thrash-machine template: the subtitle
// marker + start.sh, and — the important part — the committed mod.json
// (git HEAD) still matching the working copy. start.sh never rewrites the
// subtitle, so the marker alone would keep showing the init panel on
// already-initialized projects; comparing id/name against HEAD matches
// start.sh's own "already initialized?" logic.
func detectTemplate(modRoot, engineRoot string) *templateInfo {
	if !fileExists(filepath.Join(modRoot, "start.sh")) {
		return nil
	}
	data, err := os.ReadFile(filepath.Join(modRoot, "mod.json"))
	if err != nil {
		return nil
	}
	var m struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		Subtitle string `json:"subtitle"`
		Chapter  int    `json:"chapter"`
	}
	if json.Unmarshal(stripJSONComments(data), &m) != nil || m.Subtitle != "Kristal Lua template" {
		return nil
	}
	// If git works, an id/name change from HEAD means the project was
	// already initialized — hide the panel (best effort; without git we
	// fall back to the subtitle check).
	if headID, headName, ok := gitHeadModJSON(modRoot); ok {
		if headID != m.ID || headName != m.Name {
			return nil
		}
	}
	info := &templateInfo{
		IsTemplate: true,
		Name:       filepath.Base(modRoot),
		Chapter:    m.Chapter,
	}
	keyRe := regexp.MustCompile(`"[A-Za-z][A-Za-z0-9_]*"\s*:`)
	for n := 1; n <= 4; n++ {
		if data, err := os.ReadFile(filepath.Join(engineRoot, "configs", fmt.Sprintf("chapter%d.json", n))); err == nil {
			info.Chapters = append(info.Chapters, chapterInfo{Number: n, Items: len(keyRe.FindAll(data, -1))})
		}
	}
	return info
}

// gitHeadModJSON reads the committed mod.json's id/name via
// `git show HEAD:mod.json` (same probe start.sh uses).
func gitHeadModJSON(modRoot string) (id, name string, ok bool) {
	out, err := exec.Command("git", "-C", modRoot, "show", "HEAD:mod.json").Output()
	if err != nil {
		return "", "", false
	}
	var m struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if json.Unmarshal(stripJSONComments(out), &m) != nil {
		return "", "", false
	}
	return m.ID, m.Name, true
}

// validProjectName restricts names to letters/digits/space/dash/underscore
// (start.sh uses the name in file renames and mod.json patches).
var projectNameRe = regexp.MustCompile(`^[\p{L}\p{N}][\p{L}\p{N} _-]{0,63}$`)

type templateInitRequest struct {
	Name string `json:"name"`
}

func (s *Server) handleTemplateInit(w http.ResponseWriter, r *http.Request) {
	var req templateInitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad request: "+err.Error())
		return
	}
	if !projectNameRe.MatchString(req.Name) {
		writeError(w, http.StatusBadRequest, "invalid project name (letters, digits, space, dash, underscore; max 64)")
		return
	}
	if info := detectTemplate(s.opts.ModRoot, s.opts.EngineRoot); info == nil || !info.IsTemplate {
		writeError(w, http.StatusBadRequest, "not a thrash-machine template")
		return
	}
	argv := []string{"bash", filepath.Join(s.opts.ModRoot, "start.sh"), "--name", req.Name}
	cmdStr := strings.Join(quoteAll(argv), " ")
	if err := s.opts.Spawn("initialize project", argv, s.opts.ModRoot, true); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"id": s.runs.add("initialize project", cmdStr)})
}

type chapterRequest struct {
	Chapter int `json:"chapter"`
}

// handleTemplateChapter rewrites mod.json's "chapter" field in place,
// preserving JSONC comments (textual replace, no JSON round-trip).
func (s *Server) handleTemplateChapter(w http.ResponseWriter, r *http.Request) {
	var req chapterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad request: "+err.Error())
		return
	}
	if req.Chapter < 1 || req.Chapter > 4 {
		writeError(w, http.StatusBadRequest, "chapter must be 1-4")
		return
	}
	path := filepath.Join(s.opts.ModRoot, "mod.json")
	data, err := os.ReadFile(path)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	re := regexp.MustCompile(`("chapter"\s*:\s*)[0-9]+`)
	if !re.Match(data) {
		writeError(w, http.StatusBadRequest, "chapter field not found in mod.json")
		return
	}
	out := re.ReplaceAll(data, []byte(fmt.Sprintf("${1}%d", req.Chapter)))
	if err := os.WriteFile(path, out, 0o644); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
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
	// The project's own build recipes (mod root justfile) as a second
	// group, minus recipes already listed from the library justfile (the
	// mod root's run/test/gui delegate to it and would just duplicate).
	var mod *tasklist.List
	if jf := filepath.Join(s.opts.ModRoot, "justfile"); fileExists(jf) {
		if m := tasklist.Load(s.opts.JustPath, jf, s.opts.ModRoot); m.Source != "builtin" {
			seen := map[string]bool{}
			for _, t := range l.Tasks {
				seen[t.Name] = true
			}
			kept := m.Tasks[:0]
			for _, t := range m.Tasks {
				if !seen[t.Name] {
					kept = append(kept, t)
				}
			}
			m.Tasks = kept
			if len(m.Tasks) > 0 {
				mod = m
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"source": l.Source,
		"note":   l.Note,
		"tasks":  l.Tasks,
		"mod":    mod,
	})
}

type runRequest struct {
	Task string   `json:"task"`
	Args []string `json:"args"`
	// Justfile selects which justfile the task runs against: "" or
	// "library" (default) uses the debug-tools justfile; "project" uses the
	// mod root justfile (build/package recipes).
	Justfile string `json:"justfile"`
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
	justfile := s.opts.Justfile
	if req.Justfile == "project" {
		justfile = filepath.Join(s.opts.ModRoot, "justfile")
		if !fileExists(justfile) {
			writeError(w, http.StatusBadRequest, "project justfile not found")
			return
		}
	}
	argv := []string{s.opts.JustPath, "--justfile", justfile, req.Task}
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
