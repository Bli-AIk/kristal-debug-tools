// Package tasklist enumerates the recipes of a justfile for the GUI.
//
// Primary source: `just --dump --dump-format json` (stable enough in 1.58
// without --unstable). Any parse failure drops through to a text parse of
// `just --list`, and finally to an empty built-in list — the GUI must never
// crash because the just output shape changed.
package tasklist

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"sort"
	"strings"
)

// Param describes one recipe parameter.
type Param struct {
	Name    string `json:"name"`
	Kind    string `json:"kind"` // "star" | "singleton" | "many" | "flag"
	Flag    bool   `json:"flag"`
	Default any    `json:"default,omitempty"`
	Min     *int   `json:"min,omitempty"`
	Max     *int   `json:"max,omitempty"`
}

// Task is a runnable recipe.
type Task struct {
	Name    string  `json:"name"`
	Doc     string  `json:"doc,omitempty"`
	Private bool    `json:"private,omitempty"`
	Params  []Param `json:"params,omitempty"`
	Aliases []string `json:"aliases,omitempty"`
}

// List is the result of enumerating a justfile.
type List struct {
	Source string `json:"source"` // "dump" | "list" | "builtin"
	Note   string `json:"note,omitempty"`
	Tasks  []Task `json:"tasks"`
}

// Builtin is the fallback list when just cannot be run at all; the GUI then
// only offers the launch-game panel.
func Builtin(reason string) *List {
	return &List{Source: "builtin", Note: reason}
}

// dumpRecipe mirrors the recipe entries of `just --dump --dump-format json`.
type dumpRecipe struct {
	Name       string      `json:"name"`
	Doc        string      `json:"doc"`
	Private    bool        `json:"private"`
	Parameters []dumpParam `json:"parameters"`
}

type dumpParam struct {
	Name    string `json:"name"`
	Kind    string `json:"kind"`
	Flag    bool   `json:"flag"`
	Default any    `json:"default"`
	Min     *int   `json:"min"`
	Max     *int   `json:"max"`
}

// ParseDumpJSON parses `just --dump --dump-format json` output.
func ParseDumpJSON(data []byte) (*List, error) {
	var dump struct {
		Recipes map[string]dumpRecipe `json:"recipes"`
		Aliases map[string]struct {
			Name   string `json:"name"`
			Target string `json:"target"`
		} `json:"aliases"`
	}
	if err := json.Unmarshal(data, &dump); err != nil {
		return nil, err
	}
	if len(dump.Recipes) == 0 {
		return nil, fmt.Errorf("dump contains no recipes")
	}

	aliasTarget := map[string]string{}
	for name, a := range dump.Aliases {
		if a.Target != "" {
			aliasTarget[a.Target] = appendUnique(aliasTarget[a.Target], name)
		}
	}

	var tasks []Task
	for name, r := range dump.Recipes {
		t := Task{
			Name:    r.Name,
			Doc:     r.Doc,
			Private: r.Private,
			Aliases: strings.Fields(aliasTarget[name]),
		}
		for _, p := range r.Parameters {
			t.Params = append(t.Params, Param{
				Name: p.Name, Kind: p.Kind, Flag: p.Flag,
				Default: p.Default, Min: p.Min, Max: p.Max,
			})
		}
		if t.Name == "" {
			t.Name = name
		}
		tasks = append(tasks, t)
	}
	sort.Slice(tasks, func(i, j int) bool { return tasks[i].Name < tasks[j].Name })
	return &List{Source: "dump", Tasks: tasks}, nil
}

func appendUnique(s, v string) string {
	for _, e := range strings.Fields(s) {
		if e == v {
			return s
		}
	}
	return strings.TrimSpace(s + " " + v)
}

// ParseList parses `just --list` output as a degraded fallback: names,
// star params and doc comments only. Lines look like
//
//	    run *args         # [alias: l]
//	    build (target)    # Build a thing
//	    test
func ParseList(data []byte) *List {
	var tasks []Task
	for _, line := range strings.Split(string(data), "\n") {
		s := strings.TrimSpace(line)
		if s == "" || strings.HasPrefix(s, "Available") || strings.HasPrefix(s, "Recipes") {
			continue
		}
		// Split off the # comment (doc or alias marker).
		doc := ""
		if i := strings.Index(s, "#"); i >= 0 {
			doc = strings.TrimSpace(s[i+1:])
			s = strings.TrimSpace(s[:i])
		}
		name, params := splitNameParams(s)
		if name == "" {
			continue
		}
		t := Task{Name: name, Doc: doc}
		if strings.Contains(doc, "[alias:") {
			// "run *args # [alias: l]" — the alias is reported on the target
			// recipe line; move it out of the doc text.
			if i := strings.Index(doc, "[alias:"); i >= 0 {
				t.Aliases = strings.Fields(strings.TrimSuffix(strings.TrimSpace(doc[i+len("[alias:"):]), "]"))
				t.Doc = strings.TrimSpace(doc[:i])
			}
		}
		if params != "" {
			if strings.HasPrefix(params, "*") {
				t.Params = []Param{{Name: params[1:], Kind: "star"}}
			} else {
				for _, p := range strings.Split(params, ",") {
					if p = strings.TrimSpace(p); p != "" {
						t.Params = append(t.Params, Param{Name: p, Kind: "singleton"})
					}
				}
			}
		}
		tasks = append(tasks, t)
	}
	return &List{Source: "list", Tasks: tasks}
}

// splitNameParams splits "run *args" into ("run", "*args") and
// "build (target, force)" into ("build", "target, force").
func splitNameParams(s string) (name, params string) {
	if i := strings.Index(s, "("); i >= 0 {
		if j := strings.Index(s, ")"); j > i {
			return strings.TrimSpace(s[:i]), strings.TrimSpace(s[i+1 : j])
		}
	}
	if i := strings.IndexAny(s, " \t"); i >= 0 {
		return s[:i], strings.TrimSpace(s[i+1:])
	}
	return s, ""
}

// Load enumerates the recipes of justfilePath, running the given just
// executable with dir as the working directory (so invocation_directory()
// matches the GUI's context). It never fails: worst case it returns the
// built-in empty list with a note.
func Load(justPath, justfilePath, dir string) *List {
	var firstErr error
	if out, err := run(justPath, justfilePath, dir, "--dump", "--dump-format", "json"); err == nil {
		if l, perr := ParseDumpJSON(out); perr == nil {
			return l
		} else {
			firstErr = fmt.Errorf("dump parse: %v", perr)
		}
	} else {
		firstErr = err
	}
	// Fallback: human-readable list.
	if out, err := run(justPath, justfilePath, dir, "--list"); err == nil {
		return ParseList(out)
	} else if firstErr == nil {
		firstErr = err
	}
	return Builtin(firstErr.Error())
}

func run(justPath, justfilePath, dir string, args ...string) ([]byte, error) {
	full := append([]string{"--justfile", justfilePath}, args...)
	cmd := exec.Command(justPath, full...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("%s %s: %v", justPath, strings.Join(full, " "), err)
	}
	return out, nil
}
