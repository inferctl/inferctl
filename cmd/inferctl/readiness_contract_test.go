package main

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/inferctl/inferctl/internal/envelope"
	"github.com/inferctl/inferctl/pkg/inferctl"
)

func TestPromptMetadataRedactsFilePathAndHashesContent(t *testing.T) {
	promptPath := filepath.Join(t.TempDir(), "secret-prompt.txt")
	if err := os.WriteFile(promptPath, []byte("hello world"), 0o600); err != nil {
		t.Fatal(err)
	}

	cmd := newRootCommand()
	meta, errObj := readPromptMetadata(cmd.Command, promptReadOptions{file: promptPath, includeHash: true})
	if errObj != nil {
		t.Fatalf("read prompt metadata error = %#v", errObj)
	}
	if meta.SourceKind != "file" || meta.Source != "file" {
		t.Fatalf("source fields = %#v", meta)
	}
	if meta.Filename == nil || *meta.Filename != "secret-prompt.txt" || meta.Basename == nil || *meta.Basename != "secret-prompt.txt" {
		t.Fatalf("filename fields = %#v", meta)
	}
	if strings.Contains(meta.Source, promptPath) {
		t.Fatalf("metadata source leaked path: %#v", meta)
	}
	wantSum := sha256.Sum256([]byte("hello world"))
	wantHash := hex.EncodeToString(wantSum[:])
	if meta.ContentSHA256 == nil || *meta.ContentSHA256 != wantHash {
		t.Fatalf("content hash = %v want %s", meta.ContentSHA256, wantHash)
	}
	if meta.PromptChars != 11 || meta.EstimatedTokens != 3 {
		t.Fatalf("token estimate = %#v", meta)
	}
}

func TestPromptMetadataRejectsMultipleSources(t *testing.T) {
	cmd := newRootCommand()
	_, errObj := readPromptMetadata(cmd.Command, promptReadOptions{inline: "hello", fromStdin: true})
	if errObj == nil {
		t.Fatal("expected multiple-source error")
	}
	if errObj.Code != "E_INVALID_ARG" || errObj.ExitCode != exitUserInput {
		t.Fatalf("error = %#v", errObj)
	}
}

func TestControlPlaneChangeClassifierRanksDomainChanges(t *testing.T) {
	before := newControlPlaneSnapshot("code", promptMetadata{SourceKind: "none", Source: "none"})
	before.RouteDecision = inferctl.RouteDecision{SelectedBackend: "ollama", SelectedModel: "primary:70b", Ready: true}
	before.BackendReachability = []backendReachability{{Name: "ollama", Reachable: true}}
	before.LoadedModels = []inferctl.LoadedModelInfo{{Name: "primary:70b", Backend: "ollama"}}

	after := newControlPlaneSnapshot("code", promptMetadata{SourceKind: "none", Source: "none"})
	after.RouteDecision = inferctl.RouteDecision{SelectedBackend: "ollama", SelectedModel: "fallback:8b", IsFallback: true, Ready: false}
	after.BackendReachability = []backendReachability{{Name: "ollama", Reachable: false}}
	after.Warnings = []envelope.Warning{{Code: "W_FALLBACK_USED", Message: "fallback used"}}

	changes := classifyControlPlaneChanges(before, after)
	if len(changes) < 5 {
		t.Fatalf("changes = %#v", changes)
	}
	for i, change := range changes {
		if change.Rank != i+1 {
			t.Fatalf("rank %d = %#v", i, change)
		}
	}
	if changes[0].Severity != "high" {
		t.Fatalf("first change should be high severity: %#v", changes)
	}
	seen := map[string]bool{}
	for _, change := range changes {
		seen[change.Type] = true
	}
	for _, typ := range []string{"selected_route", "fallback_status", "backend_reachability", "selected_model_readiness", "warning_codes", "loaded_model_count"} {
		if !seen[typ] {
			t.Fatalf("missing change type %s in %#v", typ, changes)
		}
	}
}

func TestExitCodeConstantsMatchContract(t *testing.T) {
	tests := map[int]string{
		exitSuccess:     "success",
		exitUserInput:   "user_input_error",
		exitSafetyBlock: "safety_block",
		exitEnvironment: "tool_environment_error",
		exitTransient:   "transient_failure",
		exitConflict:    "conflict",
	}
	for code, name := range tests {
		if got := exitCodeName(code); got != name {
			t.Fatalf("exit code %d name = %s want %s", code, got, name)
		}
	}
}
