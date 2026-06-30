package main

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/inferctl/inferctl/internal/envelope"
)

const defaultSnapshotRetentionLimit = 20

type snapshotStoreResult struct {
	Path   string   `json:"path"`
	Pruned []string `json:"pruned"`
}

type storedSnapshotCandidate struct {
	Path     string
	Snapshot controlPlaneSnapshot
}

func snapshotStoreDir(env map[string]string) (string, *envelope.Error) {
	if env == nil {
		env = envMap()
	}
	if dir := env["INFERCTL_SNAPSHOT_DIR"]; dir != "" {
		return dir, nil
	}
	if dir := env["XDG_STATE_HOME"]; dir != "" {
		return filepath.Join(dir, "inferctl", "snapshots"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		errObj := envelope.Error{
			Code:      "E_CONFIG_UNREADABLE",
			Message:   "could not resolve snapshot store directory: " + err.Error(),
			ExitCode:  exitEnvironment,
			Retryable: false,
			Details:   map[string]any{"reason": err.Error()},
		}
		return "", &errObj
	}
	return filepath.Join(home, ".local", "state", "inferctl", "snapshots"), nil
}

func storeSnapshot(snapshot controlPlaneSnapshot, retentionLimit int, env map[string]string) (snapshotStoreResult, *envelope.Error) {
	if retentionLimit <= 0 {
		err := invalidArg("--retention-limit", fmt.Sprint(retentionLimit), "integer >= 1", nil)
		return snapshotStoreResult{}, &err
	}
	root, errObj := snapshotStoreDir(env)
	if errObj != nil {
		return snapshotStoreResult{}, errObj
	}
	taskDir := filepath.Join(root, safeSnapshotTask(snapshot.Task))
	if err := os.MkdirAll(taskDir, 0o700); err != nil {
		errObj := envelope.Error{
			Code:      "E_CONFIG_WRITE_FAILED",
			Message:   "could not create snapshot store at " + taskDir + ": " + err.Error(),
			ExitCode:  exitEnvironment,
			Retryable: false,
			Details:   map[string]any{"path": taskDir, "reason": err.Error()},
		}
		return snapshotStoreResult{}, &errObj
	}
	name := safeSnapshotFilename(snapshot.CapturedAtISO)
	path := filepath.Join(taskDir, name+".json")
	for i := 1; ; i++ {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			break
		}
		path = filepath.Join(taskDir, fmt.Sprintf("%s-%02d.json", name, i))
	}
	if errObj := writeSnapshotArtifact(path, snapshot); errObj != nil {
		return snapshotStoreResult{}, errObj
	}
	pruned, errObj := pruneSnapshotStore(taskDir, retentionLimit)
	if errObj != nil {
		return snapshotStoreResult{}, errObj
	}
	return snapshotStoreResult{Path: path, Pruned: pruned}, nil
}

func pruneSnapshotStore(taskDir string, retentionLimit int) ([]string, *envelope.Error) {
	candidates, errObj := listStoredSnapshots(taskDir)
	if errObj != nil {
		return nil, errObj
	}
	if len(candidates) <= retentionLimit {
		return []string{}, nil
	}
	slices.SortFunc(candidates, func(a, b storedSnapshotCandidate) int {
		if a.Snapshot.CapturedAtISO == b.Snapshot.CapturedAtISO {
			return strings.Compare(a.Path, b.Path)
		}
		return strings.Compare(a.Snapshot.CapturedAtISO, b.Snapshot.CapturedAtISO)
	})
	var pruned []string
	for _, candidate := range candidates[:len(candidates)-retentionLimit] {
		if err := os.Remove(candidate.Path); err != nil {
			errObj := envelope.Error{
				Code:      "E_CONFIG_WRITE_FAILED",
				Message:   "could not prune snapshot " + candidate.Path + ": " + err.Error(),
				ExitCode:  exitEnvironment,
				Retryable: false,
				Details:   map[string]any{"path": candidate.Path, "reason": err.Error()},
			}
			return nil, &errObj
		}
		pruned = append(pruned, candidate.Path)
	}
	return pruned, nil
}

func selectStoredSnapshot(task, since string, env map[string]string, now time.Time) (storedSnapshotCandidate, *envelope.Error) {
	root, errObj := snapshotStoreDir(env)
	if errObj != nil {
		return storedSnapshotCandidate{}, errObj
	}
	taskDir := filepath.Join(root, safeSnapshotTask(task))
	candidates, errObj := listStoredSnapshots(taskDir)
	if errObj != nil {
		return storedSnapshotCandidate{}, errObj
	}
	if len(candidates) == 0 {
		err := invalidArg("--since", since, "at least one stored snapshot for task "+task, nil)
		return storedSnapshotCandidate{}, &err
	}
	cutoff, err := parseSinceCutoff(now, since)
	if err != nil {
		errObj := invalidArg("--since", since, "duration like 24h, RFC3339 timestamp, YYYY-MM-DD, or yesterday", nil)
		return storedSnapshotCandidate{}, &errObj
	}
	var matches []storedSnapshotCandidate
	for _, candidate := range candidates {
		captured, err := time.Parse(time.RFC3339Nano, candidate.Snapshot.CapturedAtISO)
		if err != nil {
			errObj := invalidArg("captured_at_iso", candidate.Snapshot.CapturedAtISO, "RFC3339 timestamp", nil)
			return storedSnapshotCandidate{}, &errObj
		}
		if !captured.After(cutoff) {
			matches = append(matches, candidate)
		}
	}
	if len(matches) == 0 {
		err := invalidArg("--since", since, "a stored snapshot at or before the requested cutoff", nil)
		return storedSnapshotCandidate{}, &err
	}
	slices.SortFunc(matches, func(a, b storedSnapshotCandidate) int {
		if a.Snapshot.CapturedAtISO == b.Snapshot.CapturedAtISO {
			return strings.Compare(a.Path, b.Path)
		}
		return strings.Compare(b.Snapshot.CapturedAtISO, a.Snapshot.CapturedAtISO)
	})
	if len(matches) > 1 && matches[0].Snapshot.CapturedAtISO == matches[1].Snapshot.CapturedAtISO {
		err := invalidArg("--since", since, "one best baseline snapshot; multiple snapshots share "+matches[0].Snapshot.CapturedAtISO, nil)
		return storedSnapshotCandidate{}, &err
	}
	return matches[0], nil
}

func listStoredSnapshots(taskDir string) ([]storedSnapshotCandidate, *envelope.Error) {
	entries, err := os.ReadDir(taskDir)
	if os.IsNotExist(err) {
		return []storedSnapshotCandidate{}, nil
	}
	if err != nil {
		errObj := envelope.Error{
			Code:      "E_CONFIG_UNREADABLE",
			Message:   "could not read snapshot store at " + taskDir + ": " + err.Error(),
			ExitCode:  exitEnvironment,
			Retryable: false,
			Details:   map[string]any{"path": taskDir, "reason": err.Error()},
		}
		return nil, &errObj
	}
	var out []storedSnapshotCandidate
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		path := filepath.Join(taskDir, entry.Name())
		raw, err := os.ReadFile(path)
		if err != nil {
			errObj := envelope.Error{
				Code:      "E_CONFIG_UNREADABLE",
				Message:   "could not read stored snapshot " + path + ": " + err.Error(),
				ExitCode:  exitEnvironment,
				Retryable: false,
				Details:   map[string]any{"path": path, "reason": err.Error()},
			}
			return nil, &errObj
		}
		snapshot, err := decodeControlPlaneSnapshot(raw)
		if err != nil {
			errObj := envelope.Error{
				Code:      "E_CONFIG_INVALID",
				Message:   "stored snapshot at " + path + " is malformed: " + err.Error(),
				ExitCode:  exitEnvironment,
				Retryable: false,
				Details:   map[string]any{"path": path, "reason": err.Error()},
			}
			return nil, &errObj
		}
		out = append(out, storedSnapshotCandidate{Path: path, Snapshot: snapshot})
	}
	return out, nil
}

func parseSinceCutoff(now time.Time, raw string) (time.Time, error) {
	if raw == "yesterday" {
		return now.Add(-24 * time.Hour), nil
	}
	if d, err := time.ParseDuration(raw); err == nil {
		return now.Add(-d), nil
	}
	if t, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		return t, nil
	}
	if t, err := time.Parse("2006-01-02", raw); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("unsupported relative time")
}

func safeSnapshotTask(task string) string {
	var b strings.Builder
	for _, r := range task {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			b.WriteRune(r)
			continue
		}
		b.WriteByte('-')
	}
	out := strings.Trim(b.String(), "-.")
	if out == "" {
		return "task"
	}
	return out
}

func safeSnapshotFilename(ts string) string {
	replacer := strings.NewReplacer(":", "-", "/", "-", "\\", "-", " ", "_")
	return strings.TrimSuffix(replacer.Replace(ts), "Z") + "Z"
}
