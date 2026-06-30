package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/inferctl/inferctl/internal/contract"
	"github.com/inferctl/inferctl/internal/envelope"
	"github.com/inferctl/inferctl/internal/render"
	internalversion "github.com/inferctl/inferctl/internal/version"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func main() {
	if err := newRootCommand().Execute(); err != nil {
		var ee exitError
		if errors.As(err, &ee) {
			os.Exit(int(ee))
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

type rootCommand struct {
	*cobra.Command
	args    []string
	argsSet bool
	json    *bool
}

func (c *rootCommand) SetArgs(args []string) {
	c.args = append([]string{}, args...)
	c.argsSet = true
	c.Command.SetArgs(args)
}

func (c *rootCommand) Execute() error {
	args := os.Args[1:]
	if c.argsSet {
		args = c.args
	}
	if err := c.redirectRemovedVerb(args); err != nil {
		return err
	}
	if err := c.unknownTopLevelVerb(args); err != nil {
		return err
	}
	return c.Command.Execute()
}

func newRootCommand() *rootCommand {
	var jsonFlag bool
	base := &cobra.Command{
		Use:           "inferctl [command]",
		Short:         "Explain your local LLM stack",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			mode := render.SelectMode(render.Options{JSONFlag: jsonFlag, Env: envMap()})
			if mode == render.ModeJSON {
				return writeError(cmd, true, noVerbError())
			}
			return cmd.Help()
		},
	}
	root := &rootCommand{Command: base, json: &jsonFlag}
	root.PersistentFlags().BoolVar(&jsonFlag, "json", false, "emit JSON envelope")
	root.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
		args := os.Args[1:]
		if root.argsSet {
			args = root.args
		}
		return writeError(cmd, jsonFlag || jsonRequested(args), unknownFlagError(cmd, err))
	})
	root.AddCommand(newCapabilitiesCommand(&jsonFlag))
	root.AddCommand(newConfigCommand(&jsonFlag))
	root.AddCommand(newBackendsCommand(&jsonFlag))
	root.AddCommand(newModelsCommand(&jsonFlag))
	root.AddCommand(newModelCommand(&jsonFlag))
	root.AddCommand(newDoctorCommand(&jsonFlag))
	root.AddCommand(newRouteCommand(&jsonFlag))
	root.AddCommand(newDiscoverCommand(&jsonFlag))
	root.AddCommand(newTriageCommand(&jsonFlag))
	root.AddCommand(newVersionCommand(&jsonFlag))
	root.AddCommand(newSchemaCommand(&jsonFlag))
	root.AddCommand(newRobotDocsCommand(&jsonFlag))
	applyHelpTemplate(root.Command)
	return root
}

const helpTemplate = `{{with (or .Long .Short)}}{{. | trimTrailingWhitespaces}}

{{end}}Usage:
  {{.UseLine}}{{if .HasAvailableSubCommands}}

Available Commands:
{{range .Commands}}{{if (or .IsAvailableCommand (eq .Name "help"))}}  {{rpad .Name .NamePadding }} {{.Short}}
{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}
Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}
{{end}}{{if .HasAvailableInheritedFlags}}
Global Flags:
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}
{{end}}
Exit Codes:
  0 success; 1 user-input; 2 safety; 3 environment; 4 transient; 5 conflict.
  See: inferctl capabilities --json

Agent/Automation:
  Machine contract: inferctl capabilities --json
  Workflow guide: inferctl robot-docs guide
  JSON envelope: add --json or set INFERCTL_FORMAT=json
`

func applyHelpTemplate(cmd *cobra.Command) {
	cmd.SetHelpTemplate(helpTemplate)
	for _, child := range cmd.Commands() {
		applyHelpTemplate(child)
	}
}

func (c *rootCommand) redirectRemovedVerb(args []string) error {
	index, verb, ok := firstVerb(args)
	if !ok {
		return nil
	}
	switch verb {
	case "explain":
		tail := append([]string{}, args[index+1:]...)
		newCommand := "inferctl route"
		if len(tail) == 0 {
			newCommand += " <task>"
		} else {
			newCommand += " " + strings.Join(tail, " ")
		}
		if !slices.Contains(tail, "--explain") {
			newCommand += " --explain"
		}
		return writeError(c.Command, jsonRequested(args) || *c.json, renamedVerbError("explain", newCommand))
	case "capabilities":
		model, hasModel := firstPositionalAfter(args[index+1:])
		if !hasModel {
			return nil
		}
		newCommand := "inferctl model " + model
		if slices.Contains(args, "--json") {
			newCommand += " --json"
		}
		return writeError(c.Command, jsonRequested(args) || *c.json, renamedVerbError("capabilities", newCommand))
	case "config":
		subcommand, hasSubcommand := firstPositionalAfter(args[index+1:])
		if !hasSubcommand || subcommand != "valid" {
			return nil
		}
		newCommand := "inferctl config validate"
		if slices.Contains(args, "--json") {
			newCommand += " --json"
		}
		return writeError(c.Command, jsonRequested(args) || *c.json, renamedVerbError("config valid", newCommand))
	default:
		return nil
	}
}

func (c *rootCommand) unknownTopLevelVerb(args []string) error {
	_, verb, ok := firstVerb(args)
	if !ok || slices.Contains(rootVerbNames(), verb) {
		return nil
	}
	errObj := unknownVerbError(verb)
	return writeError(c.Command, jsonRequested(args) || *c.json, errObj)
}

func newCapabilitiesCommand(jsonFlag *bool) *cobra.Command {
	return &cobra.Command{
		Use:   "capabilities",
		Short: "Emit the machine-readable CLI contract",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := contract.CapabilitiesData()
			if err != nil {
				return err
			}
			mode := render.SelectMode(render.Options{JSONFlag: *jsonFlag, Env: envMap()})
			if mode == render.ModeJSON {
				raw, err := contract.CapabilitiesRaw()
				if err != nil {
					return err
				}
				start := time.Now()
				env, err := envelope.New(resolvedToolVersion(), json.RawMessage(raw), envelope.Options{
					StartedAt:  start,
					FinishedAt: time.Now(),
					Env:        envMap(),
				})
				if err != nil {
					return err
				}
				return render.WriteJSON(cmd.OutOrStdout(), env)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "inferctl capabilities")
			fmt.Fprintf(cmd.OutOrStdout(), "tool: %s\n", data["tool"])
			fmt.Fprintf(cmd.OutOrStdout(), "binary: %s\n", data["binary"])
			fmt.Fprintf(cmd.OutOrStdout(), "contract: %s\n", data["contract_version"])
			return nil
		},
	}
}

type exitError int

func (e exitError) Error() string {
	return fmt.Sprintf("exit %d", int(e))
}

const (
	exitSuccess     = 0
	exitUserInput   = 1
	exitSafetyBlock = 2
	exitEnvironment = 3
	exitTransient   = 4
	exitConflict    = 5
)

func envMap() map[string]string {
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

func resolvedToolVersion() string {
	return internalversion.Tool()
}

func writeData(cmd *cobra.Command, jsonFlag bool, data any, human func() error) error {
	return writeDataWithDiagnostics(cmd, jsonFlag, data, nil, nil, human)
}

func writeDataWithDiagnostics(cmd *cobra.Command, jsonFlag bool, data any, warnings []envelope.Warning, commands []envelope.Command, human func() error) error {
	mode := render.SelectMode(render.Options{JSONFlag: jsonFlag, Env: envMap()})
	if mode == render.ModeJSON {
		start := time.Now()
		env, err := envelope.New(resolvedToolVersion(), data, envelope.Options{
			StartedAt:  start,
			FinishedAt: time.Now(),
			Env:        envMap(),
			Warnings:   warnings,
			Commands:   commands,
		})
		if err != nil {
			return err
		}
		return render.WriteJSON(cmd.OutOrStdout(), env)
	}
	return human()
}

func writeError(cmd *cobra.Command, jsonFlag bool, errObj envelope.Error) error {
	mode := render.SelectMode(render.Options{JSONFlag: jsonFlag, Env: envMap()})
	if mode == render.ModeJSON {
		start := time.Now()
		env, err := envelope.New[any](resolvedToolVersion(), nil, envelope.Options{
			StartedAt:  start,
			FinishedAt: time.Now(),
			Env:        envMap(),
			Errors:     []envelope.Error{errObj},
		})
		if err != nil {
			return err
		}
		if err := render.WriteJSON(cmd.OutOrStdout(), env); err != nil {
			return err
		}
	}
	writeErrorDiagnostic(cmd, errObj)
	return exitError(errObj.ExitCode)
}

func writeErrorDiagnostic(cmd *cobra.Command, errObj envelope.Error) {
	fmt.Fprintf(cmd.ErrOrStderr(), "error: %s\n", errObj.Message)
	if errObj.DidYouMean != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "try: %s\n", *errObj.DidYouMean)
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "exit: %d (%s, retryable: %v)\n", errObj.ExitCode, exitCodeName(errObj.ExitCode), errObj.Retryable)
}

func firstVerb(args []string) (int, string, bool) {
	for i, arg := range args {
		if arg == "--" {
			if i+1 < len(args) {
				return i + 1, args[i+1], true
			}
			return 0, "", false
		}
		if strings.HasPrefix(arg, "-") {
			continue
		}
		return i, arg, true
	}
	return 0, "", false
}

func firstPositionalAfter(args []string) (string, bool) {
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			continue
		}
		return arg, true
	}
	return "", false
}

func jsonRequested(args []string) bool {
	for _, arg := range args {
		if arg == "--json" {
			return true
		}
	}
	return os.Getenv("INFERCTL_FORMAT") == "json"
}

func rootVerbNames() []string {
	return []string{"doctor", "backends", "models", "model", "route", "config", "discover", "triage", "capabilities", "version", "schema", "robot-docs"}
}

func unknownVerbError(verb string) envelope.Error {
	nearest, distance := nearestString(verb, rootVerbNames())
	var did string
	var nearestValue any
	var distanceValue any
	if distance <= 2 {
		did = "inferctl " + nearest
		nearestValue = nearest
		distanceValue = distance
	} else {
		did = "inferctl --help"
		nearestValue = nil
		distanceValue = nil
	}
	return envelope.Error{
		Code:       "E_UNKNOWN_VERB",
		Message:    "unknown verb '" + verb + "'",
		DidYouMean: &did,
		ExitCode:   1,
		Retryable:  false,
		Details:    map[string]any{"given": verb, "nearest": nearestValue, "distance": distanceValue},
	}
}

func noVerbError() envelope.Error {
	did := "inferctl --help"
	return envelope.Error{
		Code:       "E_MISSING_ARG",
		Message:    "no verb specified",
		DidYouMean: &did,
		ExitCode:   1,
		Retryable:  false,
		Details:    map[string]any{"missing": "verb", "valid_verbs": rootVerbNames()},
	}
}

func renamedVerbError(old, newCommand string) envelope.Error {
	return envelope.Error{
		Code:       "E_VERB_RENAMED",
		Message:    "verb '" + old + "' has been renamed; use '" + newCommand + "'",
		DidYouMean: &newCommand,
		ExitCode:   1,
		Retryable:  false,
		Details:    map[string]any{"old": old, "new": newCommand, "removed_in": "0.2"},
	}
}

func unknownFlagError(cmd *cobra.Command, err error) envelope.Error {
	given := parseUnknownFlag(err.Error())
	verb := cmd.CommandPath()
	nearest, distance := nearestFlag(given, cmd)
	did := verb + " --help"
	var nearestValue any
	var distanceValue any
	if nearest != "" && distance <= 2 {
		did = verb + " " + nearest
		nearestValue = nearest
		distanceValue = distance
	}
	return envelope.Error{
		Code:       "E_UNKNOWN_FLAG",
		Message:    "unknown flag '" + given + "' for verb '" + verb + "'",
		DidYouMean: &did,
		ExitCode:   1,
		Retryable:  false,
		Details:    map[string]any{"verb": verb, "given": given, "nearest": nearestValue, "distance": distanceValue},
	}
}

func parseUnknownFlag(message string) string {
	if strings.HasPrefix(message, "unknown flag: ") {
		return strings.TrimPrefix(message, "unknown flag: ")
	}
	if strings.HasPrefix(message, "unknown shorthand flag: ") {
		return message
	}
	return message
}

func nearestString(given string, candidates []string) (string, int) {
	best := ""
	bestDistance := 1 << 30
	for _, candidate := range candidates {
		distance := levenshtein(given, candidate)
		if distance < bestDistance || (distance == bestDistance && candidate < best) {
			best = candidate
			bestDistance = distance
		}
	}
	return best, bestDistance
}

func nearestFlag(given string, cmd *cobra.Command) (string, int) {
	candidates := commandFlagNames(cmd)
	if len(candidates) == 0 {
		return "", 0
	}
	return nearestString(given, candidates)
}

func commandFlagNames(cmd *cobra.Command) []string {
	seen := map[string]bool{}
	var names []string
	for _, flags := range []*pflag.FlagSet{cmd.Flags(), cmd.InheritedFlags()} {
		flags.VisitAll(func(flag *pflag.Flag) {
			long := "--" + flag.Name
			if !seen[long] {
				seen[long] = true
				names = append(names, long)
			}
			if flag.Shorthand != "" {
				short := "-" + flag.Shorthand
				if !seen[short] {
					seen[short] = true
					names = append(names, short)
				}
			}
		})
	}
	slices.Sort(names)
	return names
}

func levenshtein(a, b string) int {
	ar := []rune(a)
	br := []rune(b)
	dp := make([][]int, len(ar)+1)
	for i := range dp {
		dp[i] = make([]int, len(br)+1)
		dp[i][0] = i
	}
	for j := range dp[0] {
		dp[0][j] = j
	}
	for i := 1; i <= len(ar); i++ {
		for j := 1; j <= len(br); j++ {
			cost := 0
			if ar[i-1] != br[j-1] {
				cost = 1
			}
			dp[i][j] = min(dp[i-1][j]+1, dp[i][j-1]+1, dp[i-1][j-1]+cost)
		}
	}
	return dp[len(ar)][len(br)]
}

func exitCodeName(code int) string {
	switch code {
	case exitSuccess:
		return "success"
	case exitUserInput:
		return "user_input_error"
	case exitSafetyBlock:
		return "safety_block"
	case exitEnvironment:
		return "tool_environment_error"
	case exitTransient:
		return "transient_failure"
	case exitConflict:
		return "conflict"
	default:
		return "unknown"
	}
}
