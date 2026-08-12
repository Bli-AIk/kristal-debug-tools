// Package server hosts the GUI's HTTP API and embedded frontend. It binds to
// 127.0.0.1 only; there are no arbitrary-command endpoints — runs are just
// recipes of a fixed justfile, and the game launch uses the launcher's
// whitelisted flags plus passthrough, exactly like the CLI.
package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/Bli-AIk/kristal-debug-tools/gui/internal/justbin"
	"github.com/Bli-AIk/kristal-debug-tools/gui/internal/launcher"
	"github.com/Bli-AIk/kristal-debug-tools/gui/internal/tasklist"
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
}

// Server serves the GUI.
type Server struct {
	opts Options
	mux  *http.ServeMux
	runs *runManager
}

func New(opts Options) *Server {
	s := &Server{opts: opts, runs: newRunManager()}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/status", s.handleStatus)
	mux.HandleFunc("GET /api/tasks", s.handleTasks)
	mux.HandleFunc("POST /api/tasks/refresh", s.handleTasks)
	mux.HandleFunc("POST /api/runs", s.handleRun)
	mux.HandleFunc("GET /api/runs", s.handleRuns)
	mux.HandleFunc("GET /api/runs/{id}/stream", s.handleStream)
	mux.HandleFunc("POST /api/runs/{id}/cancel", s.handleCancel)
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

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
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
		"os":   runtime.GOOS,
		"arch": runtime.GOARCH,
	})
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
	id, err := s.runs.startFromCmd(req.Task, cmdStr, argv, s.opts.ModRoot)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"id": id})
}

func (s *Server) handleRuns(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"runs": s.runs.logSnapshot()})
}

func (s *Server) handleStream(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad run id")
		return
	}
	s.runs.stream(w, r, id)
}

func (s *Server) handleCancel(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad run id")
		return
	}
	rs, ok := s.runs.get(id)
	if !ok {
		writeError(w, http.StatusNotFound, "unknown run")
		return
	}
	rs.cancel()
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
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
	argv = append(argv, req.Passthrough...)

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
	id, err := s.runs.startFromCmd("launch game", cmdStr, cmd.Args, cmd.Dir)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"id": id})
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
