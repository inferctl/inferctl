package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"github.com/pelletier/go-toml/v2/unstable"
)

type Loader struct {
	Env        map[string]string
	WorkingDir string
	HomeDir    string
}

type LoadOptions struct {
	FlagOverrides map[string]any
}

type SourcePaths struct {
	Selected   *string  `json:"selected"`
	Searched   []string `json:"searched"`
	SelectedBy *string  `json:"selected_by"`
}

type Result struct {
	Config      Config
	SourcePaths SourcePaths
	Provenance  map[string]Provenance
	Positions   map[string]Position
}

type LoadError struct {
	Code     string
	Path     string
	Reason   string
	Line     *int
	Column   *int
	Searched []string
}

func (e *LoadError) Error() string {
	if e.Path != "" {
		return fmt.Sprintf("%s: %s: %s", e.Code, e.Path, e.Reason)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Reason)
}

func (l Loader) Load(opts LoadOptions) (*Result, error) {
	env := l.env()
	source, err := l.resolve(env)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(*source.Selected)
	if err != nil {
		return nil, &LoadError{Code: "E_CONFIG_UNREADABLE", Path: *source.Selected, Reason: err.Error()}
	}
	return LoadBytes(data, source, env, opts)
}

func LoadBytes(data []byte, source SourcePaths, env map[string]string, opts LoadOptions) (*Result, error) {
	cfg := Defaults()
	if err := toml.Unmarshal(data, &cfg); err != nil {
		le := &LoadError{Code: "E_CONFIG_INVALID", Reason: err.Error()}
		if source.Selected != nil {
			le.Path = *source.Selected
		}
		var decodeErr *toml.DecodeError
		if errors.As(err, &decodeErr) {
			line, col := decodeErr.Position()
			le.Line = &line
			le.Column = &col
		}
		return nil, le
	}
	if cfg.Backends == nil {
		cfg.Backends = map[string]BackendConfig{}
	}
	if cfg.Routing == nil {
		cfg.Routing = map[string]RoutingConfig{}
	}

	positions, err := KeyPositions(data)
	if err != nil {
		return nil, &LoadError{Code: "E_CONFIG_INVALID", Reason: err.Error()}
	}

	prov := map[string]Provenance{}
	markFileProvenance(positions, prov)
	applyDefaults(&cfg, prov)
	if err := applyEnv(&cfg, env, prov); err != nil {
		return nil, err
	}
	applyFlagOverrides(&cfg, opts.FlagOverrides, prov)

	return &Result{
		Config:      cfg,
		SourcePaths: source,
		Provenance:  prov,
		Positions:   positions,
	}, nil
}

func (l Loader) resolve(env map[string]string) (SourcePaths, error) {
	wd := l.WorkingDir
	if wd == "" {
		var err error
		wd, err = os.Getwd()
		if err != nil {
			return SourcePaths{}, err
		}
	}
	home := l.HomeDir
	if home == "" {
		home, _ = os.UserHomeDir()
	}
	var candidates []struct {
		path string
		by   string
		note string
	}
	if p := strings.TrimSpace(env["INFERCTL_CONFIG"]); p != "" {
		candidates = append(candidates, struct{ path, by, note string }{p, "env", "$INFERCTL_CONFIG"})
	} else {
		candidates = append(candidates, struct{ path, by, note string }{"", "env", "$INFERCTL_CONFIG (unset)"})
	}
	candidates = append(candidates, struct{ path, by, note string }{filepath.Join(wd, "inferctl.toml"), "repo_local", "./inferctl.toml"})
	if xdg := strings.TrimSpace(env["XDG_CONFIG_HOME"]); xdg != "" {
		candidates = append(candidates, struct{ path, by, note string }{filepath.Join(xdg, "inferctl", "config.toml"), "xdg_explicit", "$XDG_CONFIG_HOME/inferctl/config.toml"})
	} else {
		candidates = append(candidates, struct{ path, by, note string }{"", "xdg_explicit", "$XDG_CONFIG_HOME/inferctl/config.toml (not set)"})
	}
	if home != "" {
		candidates = append(candidates, struct{ path, by, note string }{filepath.Join(home, ".config", "inferctl", "config.toml"), "xdg_default", filepath.Join(home, ".config", "inferctl", "config.toml")})
	}

	var searched []string
	for _, c := range candidates {
		if c.path == "" {
			searched = append(searched, c.note)
			continue
		}
		if _, err := os.Stat(c.path); err == nil {
			selected := c.path
			by := c.by
			searched = append(searched, c.path)
			return SourcePaths{Selected: &selected, Searched: searched, SelectedBy: &by}, nil
		}
		searched = append(searched, c.note+" (not found)")
	}
	return SourcePaths{Searched: searched}, &LoadError{Code: "E_CONFIG_MISSING", Reason: "no config file found", Searched: searched}
}

func (l Loader) env() map[string]string {
	if l.Env != nil {
		return l.Env
	}
	out := map[string]string{}
	for _, kv := range os.Environ() {
		k, v, ok := strings.Cut(kv, "=")
		if ok {
			out[k] = v
		}
	}
	return out
}

func applyDefaults(cfg *Config, prov map[string]Provenance) {
	if cfg.Meta.SchemaVersion == "" {
		cfg.Meta.SchemaVersion = "0.1"
	}
	if cfg.Profile.Mode == "" {
		cfg.Profile.Mode = "warn"
	}
	for name, b := range cfg.Backends {
		cfg.Backends[name] = applyBackendDefaults(b)
	}
	for task, r := range cfg.Routing {
		cfg.Routing[task] = applyRoutingDefaults(r)
	}
	markDefaultProvenance(*cfg, prov)
}

func markFileProvenance(positions map[string]Position, prov map[string]Provenance) {
	keys := make([]string, 0, len(positions))
	for key := range positions {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		prov[key] = ProvenanceFile
	}
}

func applyEnv(cfg *Config, env map[string]string, prov map[string]Provenance) error {
	if mode := strings.TrimSpace(env["INFERCTL_PROFILE_MODE"]); mode != "" {
		cfg.Profile.Mode = mode
		prov["profile.mode"] = ProvenanceEnv
	}
	name := strings.TrimSpace(env["INFERCTL_DEFAULT_BACKEND"])
	if name == "" {
		return nil
	}
	if _, ok := cfg.Backends[name]; !ok {
		configured := make([]string, 0, len(cfg.Backends))
		for backend := range cfg.Backends {
			configured = append(configured, backend)
		}
		sort.Strings(configured)
		return &LoadError{
			Code:   "E_INVALID_ARG",
			Reason: fmt.Sprintf("INFERCTL_DEFAULT_BACKEND %q is not configured; configured backends: %s", name, strings.Join(configured, ", ")),
		}
	}
	for backend, b := range cfg.Backends {
		if b.Default || backend == name {
			b.Default = backend == name
			cfg.Backends[backend] = b
			prov["backends."+backend+".default"] = ProvenanceEnv
		}
	}
	return nil
}

func applyFlagOverrides(cfg *Config, flags map[string]any, prov map[string]Provenance) {
	for key, value := range flags {
		switch key {
		case "profile.mode":
			if s, ok := value.(string); ok {
				cfg.Profile.Mode = s
				prov[key] = ProvenanceFlag
			}
		}
	}
}

func KeyPositions(data []byte) (map[string]Position, error) {
	var parser unstable.Parser
	parser.Reset(data)
	positions := map[string]Position{}
	var table []string
	for parser.NextExpression() {
		expr := parser.Expression()
		switch expr.Kind {
		case unstable.Table:
			table = keyParts(expr)
			if len(table) > 0 {
				positions[strings.Join(table, ".")] = nodePosition(&parser, expr)
			}
		case unstable.KeyValue:
			parts := append([]string{}, table...)
			parts = append(parts, keyParts(expr)...)
			if len(parts) > 0 {
				positions[strings.Join(parts, ".")] = nodePosition(&parser, firstKey(expr))
			}
		}
	}
	if err := parser.Error(); err != nil {
		return nil, err
	}
	return positions, nil
}

func keyParts(n *unstable.Node) []string {
	var parts []string
	it := n.Key()
	for it.Next() {
		parts = append(parts, string(it.Node().Data))
	}
	return parts
}

func firstKey(n *unstable.Node) *unstable.Node {
	it := n.Key()
	if it.Next() {
		return it.Node()
	}
	return n
}

func nodePosition(parser *unstable.Parser, n *unstable.Node) Position {
	shape := parser.Shape(n.Raw)
	return Position{Line: shape.Start.Line, Column: shape.Start.Column}
}

func ConfigExplanationAvailableWithoutFile() bool {
	return true
}
