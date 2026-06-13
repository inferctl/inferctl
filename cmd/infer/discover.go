package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/Ozhiaki/inferctl/internal/config"
	"github.com/Ozhiaki/inferctl/internal/envelope"
	"github.com/spf13/cobra"
)

type discoveryReport struct {
	Summary    discoverySummary     `json:"summary"`
	Scan       discoveryScan        `json:"scan"`
	Candidates []discoveryCandidate `json:"candidates"`
	Delivery   *deliveryMetadata    `json:"delivery"`
}

type deliveryMetadata struct {
	Mode          string   `json:"mode"`
	Format        string   `json:"format"`
	Written       bool     `json:"written"`
	Path          *string  `json:"path"`
	ArtifactPaths []string `json:"artifact_paths"`
	Bytes         int      `json:"bytes"`
}

type discoverySummary struct {
	ProbedCount    int `json:"probed_count"`
	ReachableCount int `json:"reachable_count"`
	VerifiedCount  int `json:"verified_count"`
	PatchableCount int `json:"patchable_count"`
}

type discoveryScan struct {
	Mode      string `json:"mode"`
	Ports     []int  `json:"ports"`
	TimeoutMS int    `json:"timeout_ms"`
}

type discoveryCandidate struct {
	Kind         string             `json:"kind"`
	NameHint     string             `json:"name_hint"`
	BaseURL      string             `json:"base_url"`
	Port         int                `json:"port"`
	Reachable    bool               `json:"reachable"`
	Verified     bool               `json:"verified"`
	Confidence   string             `json:"confidence"`
	Version      *string            `json:"version"`
	ModelsCount  *int               `json:"models_count"`
	AuthRequired bool               `json:"auth_required"`
	ConfigPatch  *string            `json:"config_patch"`
	Warnings     []envelope.Warning `json:"warnings"`
}

func newDiscoverCommand(jsonFlag *bool) *cobra.Command {
	var format string
	var kind string
	var timeoutMS int
	var deliverPath string
	cmd := &cobra.Command{
		Use:   "discover",
		Short: "Discover supported local inference backends",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if format != "text" && format != "json" && format != "toml" {
				return writeError(cmd, *jsonFlag, invalidArg("--format", format, "one of text, json, toml", []string{"text", "json", "toml"}))
			}
			if kind != "" && !slices.Contains(discoveryKinds(), kind) {
				return writeError(cmd, *jsonFlag, invalidArg("--kind", kind, "one of ollama, llama.cpp, openai_compat, lmstudio, mlx", discoveryKinds()))
			}
			if timeoutMS < 100 {
				return writeError(cmd, *jsonFlag, invalidArg("--timeout-ms", strconv.Itoa(timeoutMS), "integer >= 100", nil))
			}
			report, errObj := runDiscovery(cmd.Context(), kind, timeoutMS)
			if errObj != nil {
				return writeError(cmd, *jsonFlag, *errObj)
			}
			if deliverPath != "" {
				delivery, errObj := deliverDiscoveryArtifact(report, deliverPath)
				if errObj != nil {
					return writeError(cmd, *jsonFlag, *errObj)
				}
				report.Delivery = delivery
			}
			return writeData(cmd, *jsonFlag || format == "json", report, func() error {
				if format == "toml" {
					fmt.Fprint(cmd.OutOrStdout(), discoveryConfigPatch(report))
					return nil
				}
				for _, candidate := range report.Candidates {
					fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\tverified=%v\n", candidate.Kind, candidate.BaseURL, candidate.Verified)
				}
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&format, "format", "text", "output format: text, json, or toml")
	cmd.Flags().StringVar(&kind, "kind", "", "backend kind to probe")
	cmd.Flags().IntVar(&timeoutMS, "timeout-ms", 750, "per-probe timeout in milliseconds")
	cmd.Flags().StringVar(&deliverPath, "deliver", "", "write a TOML config patch artifact to this path")
	return cmd
}

func runDiscovery(ctx context.Context, kind string, timeoutMS int) (discoveryReport, *envelope.Error) {
	ports := discoveryPorts(kind)
	report := discoveryReport{
		Scan: discoveryScan{
			Mode:      "localhost_fixed_ports",
			Ports:     ports,
			TimeoutMS: timeoutMS,
		},
		Candidates: []discoveryCandidate{},
	}
	client := &http.Client{Timeout: time.Duration(timeoutMS) * time.Millisecond}
	for _, port := range ports {
		report.Summary.ProbedCount++
		candidates, errObj := probeDiscoveryPort(ctx, client, kind, port)
		if errObj != nil {
			return report, errObj
		}
		for _, candidate := range candidates {
			if candidate.Reachable {
				report.Summary.ReachableCount++
			}
			if candidate.Verified {
				report.Summary.VerifiedCount++
			}
			if candidate.ConfigPatch != nil {
				report.Summary.PatchableCount++
			}
			report.Candidates = append(report.Candidates, candidate)
		}
	}
	slices.SortFunc(report.Candidates, func(a, b discoveryCandidate) int {
		if a.Kind == b.Kind {
			return a.Port - b.Port
		}
		return strings.Compare(a.Kind, b.Kind)
	})
	return report, nil
}

func probeDiscoveryPort(ctx context.Context, client *http.Client, kind string, port int) ([]discoveryCandidate, *envelope.Error) {
	baseURL := "http://127.0.0.1:" + strconv.Itoa(port)
	if kind == "" || kind == "ollama" {
		if candidate, ok := probeOllama(ctx, client, baseURL, port, kind == "ollama"); ok {
			return []discoveryCandidate{candidate}, nil
		}
	}
	if kind == "ollama" {
		return nil, nil
	}
	models, status, ok := probeOpenAIModels(ctx, client, baseURL)
	if status == http.StatusUnauthorized || status == http.StatusForbidden {
		errObj := backendConfigError("discover", "E_BACKEND_AUTH_FAILED", "backend discovery authentication failed")
		return nil, errObj
	}
	if !ok {
		return nil, nil
	}
	if kind != "" {
		candidate := openAIDiscoveryCandidate(kind, baseURL, port, models)
		return []discoveryCandidate{candidate}, nil
	}
	warning := envelope.Warning{
		Code:    "W_DISCOVERY_AMBIGUOUS",
		Message: "OpenAI-compatible localhost port could match multiple backend kinds",
		Details: map[string]any{"port": port, "kinds": []string{"llama.cpp", "lmstudio", "mlx", "openai_compat"}},
	}
	var candidates []discoveryCandidate
	for _, ambiguousKind := range []string{"llama.cpp", "lmstudio", "mlx", "openai_compat"} {
		candidate := openAIDiscoveryCandidate(ambiguousKind, baseURL, port, models)
		candidate.Confidence = "medium"
		candidate.ConfigPatch = nil
		candidate.Warnings = []envelope.Warning{warning}
		candidates = append(candidates, candidate)
	}
	return candidates, nil
}

func probeOllama(ctx context.Context, client *http.Client, baseURL string, port int, forcePatch bool) (discoveryCandidate, bool) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/api/version", nil)
	if err != nil {
		return discoveryCandidate{}, false
	}
	resp, err := client.Do(req)
	if err != nil {
		return discoveryCandidate{}, false
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return discoveryCandidate{}, false
	}
	var body struct {
		Version string `json:"version"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return discoveryCandidate{}, false
	}
	version := body.Version
	candidate := discoveryCandidate{
		Kind:       "ollama",
		NameHint:   "ollama",
		BaseURL:    baseURL,
		Port:       port,
		Reachable:  true,
		Verified:   true,
		Confidence: "high",
		Version:    &version,
		Warnings:   []envelope.Warning{},
	}
	patch := configPatchForCandidate(candidate, forcePatch)
	candidate.ConfigPatch = &patch
	return candidate, true
}

func probeOpenAIModels(ctx context.Context, client *http.Client, baseURL string) (int, int, bool) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/v1/models", nil)
	if err != nil {
		return 0, 0, false
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, 0, false
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return 0, resp.StatusCode, false
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return 0, resp.StatusCode, false
	}
	var body struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return 0, resp.StatusCode, false
	}
	return len(body.Data), resp.StatusCode, true
}

func openAIDiscoveryCandidate(kind, baseURL string, port int, models int) discoveryCandidate {
	patchable := true
	candidate := discoveryCandidate{
		Kind:        kind,
		NameHint:    safeBackendName(kind),
		BaseURL:     baseURL,
		Port:        port,
		Reachable:   true,
		Verified:    true,
		Confidence:  "high",
		ModelsCount: &models,
		Warnings:    []envelope.Warning{},
	}
	if patchable {
		patch := configPatchForCandidate(candidate, true)
		candidate.ConfigPatch = &patch
	}
	return candidate
}

func configPatchForCandidate(candidate discoveryCandidate, defaultBackend bool) string {
	return fmt.Sprintf("[backends.%s]\nkind = %q\nbase_url = %q\ndefault = %t\n\n",
		candidate.NameHint, candidate.Kind, candidate.BaseURL, defaultBackend)
}

func discoveryConfigPatch(report discoveryReport) string {
	var b strings.Builder
	for _, candidate := range report.Candidates {
		if candidate.ConfigPatch != nil {
			b.WriteString(*candidate.ConfigPatch)
		}
	}
	return b.String()
}

func deliverDiscoveryArtifact(report discoveryReport, path string) (*deliveryMetadata, *envelope.Error) {
	artifact := discoveryConfigPatch(report)
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			errObj := configWriteError("E_CONFIG_WRITE_FAILED", "could not create delivery directory for "+path, config.FileErrorDetails(err))
			return nil, &errObj
		}
	}
	if err := config.AtomicWriteFile(path, []byte(artifact), 0o600); err != nil {
		errObj := configWriteError("E_CONFIG_WRITE_FAILED", "could not write delivery artifact to "+path, config.FileErrorDetails(err))
		return nil, &errObj
	}
	return &deliveryMetadata{
		Mode:          "artifact",
		Format:        "toml",
		Written:       true,
		Path:          &path,
		ArtifactPaths: []string{path},
		Bytes:         len([]byte(artifact)),
	}, nil
}

func safeBackendName(kind string) string {
	return strings.NewReplacer(".", "", "_", "").Replace(kind)
}

func discoveryKinds() []string {
	return []string{"ollama", "llama.cpp", "openai_compat", "lmstudio", "mlx"}
}

func discoveryPorts(kind string) []int {
	if override := strings.TrimSpace(envMap()["INFERCTL_TEST_DISCOVERY_PORTS"]); override != "" {
		var ports []int
		for _, raw := range strings.Split(override, ",") {
			port, err := strconv.Atoi(strings.TrimSpace(raw))
			if err == nil {
				ports = append(ports, port)
			}
		}
		slices.Sort(ports)
		return ports
	}
	portsByKind := map[string][]int{
		"ollama":        {11434},
		"lmstudio":      {1234},
		"llama.cpp":     {8080, 8090},
		"mlx":           {8080, 8081},
		"openai_compat": {1234, 8080, 8081, 8090},
	}
	if kind != "" {
		return append([]int{}, portsByKind[kind]...)
	}
	seen := map[int]bool{}
	var ports []int
	for _, byKind := range portsByKind {
		for _, port := range byKind {
			if !seen[port] {
				seen[port] = true
				ports = append(ports, port)
			}
		}
	}
	slices.Sort(ports)
	return ports
}
