package version

import (
	"runtime/debug"
	"testing"
)

func TestToolPrefersInjectedVersion(t *testing.T) {
	withTestResolver(t, "v1.2.3", "", "", nil)

	if got := Tool(); got != "1.2.3" {
		t.Fatalf("Tool() = %q, want %q", got, "1.2.3")
	}
}

func TestToolReadsTaggedBuildInfo(t *testing.T) {
	withTestResolver(t, "", "", "", &debug.BuildInfo{
		Main: debug.Module{Version: "v0.2.1"},
	})

	if got := Tool(); got != "0.2.1" {
		t.Fatalf("Tool() = %q, want %q", got, "0.2.1")
	}
}

func TestToolPreservesPseudoVersionWithoutLeadingV(t *testing.T) {
	const pseudo = "v0.2.1-0.20260614-abcdef123456"
	withTestResolver(t, "", "", "", &debug.BuildInfo{
		Main: debug.Module{Version: pseudo},
	})

	if got := Tool(); got != pseudo[1:] {
		t.Fatalf("Tool() = %q, want %q", got, pseudo[1:])
	}
}

func TestToolFallsBackToDevForDevelAndMissingBuildInfo(t *testing.T) {
	withTestResolver(t, "", "", "", &debug.BuildInfo{
		Main: debug.Module{Version: "(devel)"},
	})
	if got := Tool(); got != "dev" {
		t.Fatalf("Tool() with devel = %q, want %q", got, "dev")
	}

	withTestResolver(t, "", "", "", nil)
	if got := Tool(); got != "dev" {
		t.Fatalf("Tool() without build info = %q, want %q", got, "dev")
	}
}

func TestCommitAndDatePreferInjectedValuesThenBuildSettings(t *testing.T) {
	withTestResolver(t, "", "ldflag-commit", "2026-06-14T12:00:00Z", &debug.BuildInfo{
		Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: "buildinfo-commit"},
			{Key: "vcs.time", Value: "2026-06-10T09:30:00Z"},
		},
	})

	if got := Commit(); got != "ldflag-commit" {
		t.Fatalf("Commit() with ldflag = %q, want %q", got, "ldflag-commit")
	}
	if got := Date(); got != "2026-06-14T12:00:00Z" {
		t.Fatalf("Date() with ldflag = %q, want %q", got, "2026-06-14T12:00:00Z")
	}

	withTestResolver(t, "", "", "", &debug.BuildInfo{
		Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: "buildinfo-commit"},
			{Key: "vcs.time", Value: "2026-06-10T09:30:00Z"},
		},
	})

	if got := Commit(); got != "buildinfo-commit" {
		t.Fatalf("Commit() from build info = %q, want %q", got, "buildinfo-commit")
	}
	if got := Date(); got != "2026-06-10T09:30:00Z" {
		t.Fatalf("Date() from build info = %q, want %q", got, "2026-06-10T09:30:00Z")
	}

	withTestResolver(t, "", "", "", nil)
	if got := Commit(); got != "" {
		t.Fatalf("Commit() without build info = %q, want empty", got)
	}
	if got := Date(); got != "" {
		t.Fatalf("Date() without build info = %q, want empty", got)
	}
}

func withTestResolver(t *testing.T, tool, buildCommit, buildDate string, info *debug.BuildInfo) {
	t.Helper()

	version = tool
	commit = buildCommit
	date = buildDate
	readBuildInfo = func() (*debug.BuildInfo, bool) {
		if info == nil {
			return nil, false
		}
		return info, true
	}

	t.Cleanup(func() {
		version = ""
		commit = ""
		date = ""
		readBuildInfo = debug.ReadBuildInfo
	})
}
