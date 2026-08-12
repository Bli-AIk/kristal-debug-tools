// Chapter config: the engine's per-chapter default feature configs
// (configs/chapterN.json, selected by mod.json's "chapter"), with Chinese
// descriptions vendored from the Kristal website's configurable-features
// page, and per-key overrides written into mod.json's config.kristal block
// (JSONC-preserving — the engine reads them via Game:getConfig).
package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"

	"github.com/Bli-AIk/kristal-debug-tools/gui/internal/web"
)

// --- reading ---

type chapterConfigItem struct {
	Key      string         `json:"key"`
	Desc     string         `json:"desc,omitempty"`
	Values   map[string]any `json:"values"`   // chapter number -> default value
	Override any            `json:"override"` // nil = use the chapter default
}

func (s *Server) handleChapterConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, chapterConfig(s.opts.ModRoot, s.opts.EngineRoot))
}

func chapterConfig(modRoot, engineRoot string) map[string]any {
	// Chapter defaults from the engine.
	chapters := []map[string]any{}
	for n := 1; n <= 4; n++ {
		m := map[string]any{}
		if data, err := os.ReadFile(filepath.Join(engineRoot, "configs", fmt.Sprintf("chapter%d.json", n))); err == nil {
			json.Unmarshal(stripJSONComments(data), &m) // best effort
		}
		chapters = append(chapters, m)
	}
	// Chinese descriptions, vendored from the Kristal website.
	desc := map[string]string{}
	if data, err := web.FS.ReadFile("config-features.json"); err == nil {
		var list []struct {
			Key  string `json:"key"`
			Desc string `json:"desc"`
		}
		if json.Unmarshal(data, &list) == nil {
			for _, e := range list {
				desc[e.Key] = e.Desc
			}
		}
	}
	// Mod overrides: mod.json's config.kristal block.
	overrides := modConfigOverrides(modRoot)

	keys := map[string]bool{}
	for _, m := range chapters {
		for k := range m {
			keys[k] = true
		}
	}
	for k := range desc {
		keys[k] = true
	}
	var sorted []string
	for k := range keys {
		sorted = append(sorted, k)
	}
	sort.Strings(sorted)

	items := make([]chapterConfigItem, 0, len(sorted))
	for _, k := range sorted {
		item := chapterConfigItem{
			Key:    k,
			Desc:   desc[k],
			Values: map[string]any{},
		}
		for n, m := range chapters {
			if v, ok := m[k]; ok {
				item.Values[fmt.Sprintf("%d", n+1)] = v
			}
		}
		if v, ok := overrides[k]; ok {
			item.Override = v
		}
		items = append(items, item)
	}
	return map[string]any{
		"chapter": currentChapter(modRoot),
		"items":   items,
	}
}

func currentChapter(modRoot string) int {
	if data, err := os.ReadFile(filepath.Join(modRoot, "mod.json")); err == nil {
		var m struct {
			Chapter int `json:"chapter"`
		}
		if json.Unmarshal(stripJSONComments(data), &m) == nil {
			return m.Chapter
		}
	}
	return 2
}

// modConfigOverrides extracts mod.json's config.kristal block.
func modConfigOverrides(modRoot string) map[string]any {
	out := map[string]any{}
	data, err := os.ReadFile(filepath.Join(modRoot, "mod.json"))
	if err != nil {
		return out
	}
	var m struct {
		Config struct {
			Kristal map[string]any `json:"kristal"`
		} `json:"config"`
	}
	if json.Unmarshal(stripJSONComments(data), &m) == nil && m.Config.Kristal != nil {
		out = m.Config.Kristal
	}
	return out
}

// --- writing (JSONC-preserving) ---

type chapterConfigSetRequest struct {
	Key   string `json:"key"`
	Value any    `json:"value"` // null removes the override
}

func (s *Server) handleChapterConfigSet(w http.ResponseWriter, r *http.Request) {
	var req chapterConfigSetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad request: "+err.Error())
		return
	}
	if req.Key == "" || !configKeyRe.MatchString(req.Key) {
		writeError(w, http.StatusBadRequest, "invalid config key")
		return
	}
	if err := modConfigSet(s.opts.ModRoot, req.Key, req.Value); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

var configKeyRe = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*$`)

// modConfigSet sets config.kristal.<key> in mod.json without touching
// comments or other keys: it replaces the value portion of an existing key
// line, or inserts a new key line right after the kristal block's '{'.
// value == nil removes the key entirely.
func modConfigSet(modRoot, key string, value any) error {
	path := filepath.Join(modRoot, "mod.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	text := string(data)

	// Locate the config.kristal block with a small JSONC-aware scanner.
	configStart := findKeyValueStart(text, `"config"`)
	if configStart < 0 {
		return fmt.Errorf("config block not found in mod.json")
	}
	configEnd := findObjectEnd(text, configStart)
	kristalStart := findKeyValueStart(text[configStart:configEnd], `"kristal"`)
	if kristalStart < 0 {
		return fmt.Errorf("config.kristal block not found in mod.json")
	}
	kristalStart += configStart
	kristalEnd := findObjectEnd(text, kristalStart)

	block := text[kristalStart:kristalEnd]
	keyLineRe := regexp.MustCompile(`(?m)^([ \t]*)` + regexp.QuoteMeta(`"`+key+`"`) + `\s*:\s*[^\r\n]*\r?\n`)

	if value == nil {
		// Remove the key line (and its trailing comma if it was mid-block).
		newBlock := keyLineRe.ReplaceAllString(block, "")
		if newBlock != block {
			text = text[:kristalStart] + newBlock + text[kristalEnd:]
		}
	} else {
		jsonVal, err := json.Marshal(value)
		if err != nil {
			return err
		}
		// Update in place: replace only the value, keeping comma/comment.
		valueRe := regexp.MustCompile(`(` + regexp.QuoteMeta(`"`+key+`"`) + `\s*:\s*)[^,\r\n]*`)
		updated := valueRe.ReplaceAllString(block, "${1}"+string(jsonVal))
		if updated != block {
			text = text[:kristalStart] + updated + text[kristalEnd:]
		} else {
			// Insert after the opening '{'.
			insert := "\n    " + `"` + key + `": ` + string(jsonVal) + ","
			// keep the '{' and any immediate comment/whitespace intact
			rest := block[1:]
			text = text[:kristalStart] + block[:1] + insert + rest + text[kristalEnd:]
		}
	}
	return os.WriteFile(path, []byte(text), 0o644)
}

// findKeyValueStart returns the byte offset of the value object right after
// `"key" :`, or -1.
func findKeyValueStart(text, key string) int {
	re := regexp.MustCompile(regexp.QuoteMeta(key) + `\s*:\s*\{`)
	m := re.FindStringIndex(text)
	if m == nil {
		return -1
	}
	return m[1] - 1 // index of '{'
}

// findObjectEnd returns the index just past the matching '}' for the '{' at
// start, skipping strings and nested objects.
func findObjectEnd(text string, start int) int {
	depth := 0
	inStr, esc := false, false
	for i := start; i < len(text); i++ {
		c := text[i]
		if inStr {
			if esc {
				esc = false
			} else if c == '\\' {
				esc = true
			} else if c == '"' {
				inStr = false
			}
			continue
		}
		switch c {
		case '"':
			inStr = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i + 1
			}
		}
	}
	return len(text)
}
