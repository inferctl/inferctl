package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/Ozhiaki/inferctl/internal/testserver"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:18080", "address to listen on")
	kind := flag.String("kind", string(testserver.KindOllama), "backend kind: ollama, llama.cpp, openai_compat")
	models := flag.String("models", "qwen3:8b", "comma-separated installed model names")
	loaded := flag.String("loaded", "", "comma-separated loaded model names")
	malformed := flag.String("malformed", "", "comma-separated paths that should return malformed JSON")
	unreachable := flag.Bool("unreachable", false, "return 503 for all endpoints")
	flag.Parse()

	fixture := testserver.Fixture{
		Kind:        testserver.Kind(*kind),
		Models:      parseModels(*models),
		Loaded:      parseLoaded(*loaded),
		Malformed:   parseMalformed(*malformed),
		Unreachable: *unreachable,
		Version:     "fixture",
	}
	log.Printf("infer testserver listening on http://%s kind=%s", *addr, *kind)
	if err := http.ListenAndServe(*addr, testserver.Handler(fixture)); err != nil {
		log.Fatal(err)
	}
}

func parseModels(raw string) []testserver.Model {
	var out []testserver.Model
	for _, name := range splitList(raw) {
		out = append(out, testserver.Model{Name: name, SizeBytes: 1})
	}
	return out
}

func parseLoaded(raw string) []testserver.LoadedModel {
	var out []testserver.LoadedModel
	for _, name := range splitList(raw) {
		out = append(out, testserver.LoadedModel{Name: name, VRAMBytes: 1})
	}
	return out
}

func parseMalformed(raw string) map[string]bool {
	out := map[string]bool{}
	for _, path := range splitList(raw) {
		out[path] = true
	}
	return out
}

func splitList(raw string) []string {
	var out []string
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func init() {
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: infer-testserver [flags]\n")
		flag.PrintDefaults()
	}
}
