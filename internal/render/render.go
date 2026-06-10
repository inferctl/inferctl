package render

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

type Mode string

const (
	ModeHuman Mode = "human"
	ModeJSON  Mode = "json"
)

type Options struct {
	JSONFlag  bool
	Env       map[string]string
	StdoutTTY bool
	StderrTTY bool
}

type HumanRenderer interface {
	RenderHuman(io.Writer) error
}

func SelectMode(opts Options) Mode {
	env := opts.Env
	if env == nil {
		env = environ()
	}
	if opts.JSONFlag || env["INFERCTL_FORMAT"] == "json" {
		return ModeJSON
	}
	return ModeHuman
}

func WriteJSON(w io.Writer, value any) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return enc.Encode(value)
}

func WriteHuman(w io.Writer, data any) error {
	if r, ok := data.(HumanRenderer); ok {
		return r.RenderHuman(w)
	}
	_, err := fmt.Fprintln(w, data)
	return err
}

func ANSIAllowed(env map[string]string, isTTY bool) bool {
	if env == nil {
		env = environ()
	}
	if !isTTY {
		return false
	}
	if _, ok := env["NO_COLOR"]; ok {
		return false
	}
	if _, ok := env["CI"]; ok {
		return false
	}
	if env["TERM"] == "dumb" {
		return false
	}
	return true
}

func environ() map[string]string {
	out := map[string]string{}
	for _, kv := range os.Environ() {
		for i := 0; i < len(kv); i++ {
			if kv[i] == '=' {
				out[kv[:i]] = kv[i+1:]
				break
			}
		}
	}
	return out
}
