package version

import (
	"runtime/debug"
	"strings"
)

var (
	version = ""
	commit  = ""
	date    = ""

	readBuildInfo = debug.ReadBuildInfo
)

func Tool() string {
	if resolved := normalizeToolVersion(version); resolved != "" {
		return resolved
	}
	if info, ok := readBuildInfo(); ok {
		if resolved := normalizeToolVersion(info.Main.Version); resolved != "" {
			return resolved
		}
	}
	return "dev"
}

func Commit() string {
	if resolved := strings.TrimSpace(commit); resolved != "" {
		return resolved
	}
	return buildSetting("vcs.revision")
}

func Date() string {
	if resolved := strings.TrimSpace(date); resolved != "" {
		return resolved
	}
	return buildSetting("vcs.time")
}

func buildSetting(key string) string {
	info, ok := readBuildInfo()
	if !ok {
		return ""
	}
	for _, setting := range info.Settings {
		if setting.Key == key {
			return strings.TrimSpace(setting.Value)
		}
	}
	return ""
}

func normalizeToolVersion(raw string) string {
	candidate := strings.TrimSpace(raw)
	switch candidate {
	case "", "(devel)":
		return ""
	}
	return strings.TrimPrefix(candidate, "v")
}
