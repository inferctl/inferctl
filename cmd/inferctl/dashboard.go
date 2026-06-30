package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/inferctl/inferctl/internal/envelope"
	"github.com/spf13/cobra"
)

type dashboardOptions struct {
	interval time.Duration
}

func newDashboardCommand(jsonFlag *bool) *cobra.Command {
	opts := dashboardOptions{interval: defaultStatusWatchInterval}
	cmd := &cobra.Command{
		Use:   "dashboard",
		Short: "Run the human status dashboard backed by the public status feed",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if *jsonFlag {
				return writeError(cmd, true, invalidArg("--json", "true", "interactive dashboard; use status --json --watch for the machine feed", []string{"status --json --watch"}))
			}
			if opts.interval <= 0 {
				return writeError(cmd, *jsonFlag, invalidArg("--interval", opts.interval.String(), "positive duration such as 2s", nil))
			}
			binary, err := os.Executable()
			if err != nil {
				return err
			}
			model := newDashboardModel(statusFeedSource{
				Binary:   binary,
				Interval: opts.interval,
			})
			_, err = tea.NewProgram(model).Run()
			return err
		},
	}
	cmd.Flags().DurationVar(&opts.interval, "interval", opts.interval, "status feed polling interval")
	return cmd
}

type statusFeedSource struct {
	Binary   string
	Interval time.Duration
}

func (s statusFeedSource) args() []string {
	return dashboardStatusFeedArgs(s.Interval)
}

func dashboardStatusFeedArgs(interval time.Duration) []string {
	return []string{"status", "--json", "--watch", "--events", "--interval", interval.String()}
}

type dashboardModel struct {
	source   statusFeedSource
	feed     *dashboardFeed
	snapshot *statusSnapshot
	events   []statusEvent
	err      error
}

func newDashboardModel(source statusFeedSource) dashboardModel {
	return dashboardModel{source: source}
}

func (m dashboardModel) Init() tea.Cmd {
	return startDashboardFeedCmd(m.source)
}

func (m dashboardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			if m.feed != nil {
				_ = m.feed.stop()
			}
			return m, tea.Quit
		}
	case dashboardFeedStartedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, tea.Quit
		}
		m.feed = msg.feed
		return m, readDashboardFeedCmd(msg.feed)
	case dashboardFeedRecordMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, tea.Quit
		}
		if msg.done {
			return m, tea.Quit
		}
		if msg.snapshot != nil {
			m.snapshot = msg.snapshot
		}
		if msg.eventBatch != nil {
			m.events = append(m.events, msg.eventBatch.Events...)
			if len(m.events) > 8 {
				m.events = m.events[len(m.events)-8:]
			}
		}
		return m, readDashboardFeedCmd(m.feed)
	}
	return m, nil
}

func (m dashboardModel) View() string {
	var b strings.Builder
	b.WriteString("inferctl dashboard\n")
	b.WriteString("source: inferctl " + strings.Join(m.source.args(), " ") + "\n\n")
	if m.err != nil {
		b.WriteString("error: " + m.err.Error() + "\n")
		return b.String()
	}
	if m.snapshot == nil {
		b.WriteString("waiting for status feed...\n")
		return b.String()
	}
	s := m.snapshot.Summary
	fmt.Fprintf(&b, "backends: %d/%d reachable   models: %d loaded / %d exposed   routes: %d/%d ready   warnings: %d\n\n",
		s.BackendsReachable,
		s.BackendsTotal,
		s.ModelsLoadedTotal,
		s.ModelsExposedTotal,
		s.RoutesReady,
		s.RoutesTotal,
		s.WarningsTotal,
	)
	b.WriteString("Backends\n")
	for _, backend := range m.snapshot.Backends {
		marker := "ok"
		if !backend.Reachable {
			marker = "down"
		}
		fmt.Fprintf(&b, "  %-4s %-16s %-14s %s\n", marker, backend.Name, backend.Kind, backend.BaseURL)
	}
	b.WriteString("\nRoutes\n")
	for _, route := range m.snapshot.Routes {
		fmt.Fprintf(&b, "  %-16s -> %s/%s ready=%v fallback=%v\n",
			route.Task,
			route.Decision.SelectedBackend,
			route.Decision.SelectedModel,
			route.Decision.Ready,
			route.Decision.IsFallback,
		)
	}
	if len(m.events) > 0 {
		b.WriteString("\nEvents\n")
		for _, event := range m.events {
			fmt.Fprintf(&b, "  [%s] %s\n", event.Severity, event.Summary)
		}
	}
	b.WriteString("\nq or ctrl+c to quit\n")
	return b.String()
}

type dashboardFeed struct {
	cmd     *exec.Cmd
	scanner *bufio.Scanner
}

func (f *dashboardFeed) stop() error {
	if f == nil || f.cmd == nil || f.cmd.Process == nil {
		return nil
	}
	if err := f.cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return err
	}
	_ = f.cmd.Wait()
	return nil
}

type dashboardFeedStartedMsg struct {
	feed *dashboardFeed
	err  error
}

type dashboardFeedRecordMsg struct {
	snapshot   *statusSnapshot
	eventBatch *statusEventBatch
	err        error
	done       bool
}

func startDashboardFeedCmd(source statusFeedSource) tea.Cmd {
	return func() tea.Msg {
		cmd := exec.Command(source.Binary, source.args()...)
		cmd.Stderr = os.Stderr
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			return dashboardFeedStartedMsg{err: err}
		}
		if err := cmd.Start(); err != nil {
			return dashboardFeedStartedMsg{err: err}
		}
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
		return dashboardFeedStartedMsg{feed: &dashboardFeed{cmd: cmd, scanner: scanner}}
	}
}

func readDashboardFeedCmd(feed *dashboardFeed) tea.Cmd {
	return func() tea.Msg {
		if feed == nil || feed.scanner == nil {
			return dashboardFeedRecordMsg{err: errors.New("status feed is not running")}
		}
		if feed.scanner.Scan() {
			return dashboardRecordFromEnvelope(feed.scanner.Bytes())
		}
		if err := feed.scanner.Err(); err != nil {
			_ = feed.stop()
			return dashboardFeedRecordMsg{err: err}
		}
		if err := feed.cmd.Wait(); err != nil {
			return dashboardFeedRecordMsg{err: err}
		}
		return dashboardFeedRecordMsg{done: true}
	}
}

func dashboardRecordFromEnvelope(line []byte) dashboardFeedRecordMsg {
	var env struct {
		OK     bool             `json:"ok"`
		Data   json.RawMessage  `json:"data"`
		Errors []envelope.Error `json:"errors"`
	}
	if err := json.Unmarshal(line, &env); err != nil {
		return dashboardFeedRecordMsg{err: err}
	}
	if !env.OK {
		if len(env.Errors) > 0 {
			return dashboardFeedRecordMsg{err: errors.New(env.Errors[0].Message)}
		}
		return dashboardFeedRecordMsg{err: errors.New("status feed returned an error envelope")}
	}
	var discriminator map[string]json.RawMessage
	if err := json.Unmarshal(env.Data, &discriminator); err != nil {
		return dashboardFeedRecordMsg{err: err}
	}
	if _, ok := discriminator["status_frame_schema_version"]; ok {
		var snapshot statusSnapshot
		if err := json.Unmarshal(env.Data, &snapshot); err != nil {
			return dashboardFeedRecordMsg{err: err}
		}
		return dashboardFeedRecordMsg{snapshot: &snapshot}
	}
	if _, ok := discriminator["status_schema_version"]; ok {
		var snapshot statusSnapshot
		if err := json.Unmarshal(env.Data, &snapshot); err != nil {
			return dashboardFeedRecordMsg{err: err}
		}
		return dashboardFeedRecordMsg{snapshot: &snapshot}
	}
	if _, ok := discriminator["event_schema_version"]; ok {
		var batch statusEventBatch
		if err := json.Unmarshal(env.Data, &batch); err != nil {
			return dashboardFeedRecordMsg{err: err}
		}
		return dashboardFeedRecordMsg{eventBatch: &batch}
	}
	return dashboardFeedRecordMsg{err: errors.New("status feed emitted an unknown record type")}
}
