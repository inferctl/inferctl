package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/Ozhiaki/inferctl/internal/contract"
	"github.com/Ozhiaki/inferctl/internal/envelope"
	"github.com/Ozhiaki/inferctl/internal/render"
	"github.com/spf13/cobra"
)

const toolVersion = "0.1.0"

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
		Use:           "infer",
		Short:         "Explain your local LLM stack",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.ErrOrStderr(), "infer: no verb specified")
			return exitError(1)
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
	return root
}

func (c *rootCommand) redirectRemovedVerb(args []string) error {
	index, verb, ok := firstVerb(args)
	if !ok {
		return nil
	}
	switch verb {
	case "explain":
		tail := append([]string{}, args[index+1:]...)
		newCommand := "infer route"
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
		newCommand := "infer model " + model
		if slices.Contains(args, "--json") {
			newCommand += " --json"
		}
		return writeError(c.Command, jsonRequested(args) || *c.json, renamedVerbError("capabilities", newCommand))
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
				env, err := envelope.New(toolVersion, json.RawMessage(raw), envelope.Options{
					StartedAt:  start,
					FinishedAt: time.Now(),
					Env:        envMap(),
				})
				if err != nil {
					return err
				}
				return render.WriteJSON(cmd.OutOrStdout(), env)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "infer capabilities")
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

func writeData(cmd *cobra.Command, jsonFlag bool, data any, human func() error) error {
	return writeDataWithDiagnostics(cmd, jsonFlag, data, nil, nil, human)
}

func writeDataWithDiagnostics(cmd *cobra.Command, jsonFlag bool, data any, warnings []envelope.Warning, commands []envelope.Command, human func() error) error {
	mode := render.SelectMode(render.Options{JSONFlag: jsonFlag, Env: envMap()})
	if mode == render.ModeJSON {
		start := time.Now()
		env, err := envelope.New(toolVersion, data, envelope.Options{
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
		env, err := envelope.New[any](toolVersion, nil, envelope.Options{
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
	} else {
		fmt.Fprintf(cmd.ErrOrStderr(), "error: %s\n", errObj.Message)
		if errObj.DidYouMean != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "try: %s\n", *errObj.DidYouMean)
		}
		fmt.Fprintf(cmd.ErrOrStderr(), "exit: %d (%s, retryable: %v)\n", errObj.ExitCode, exitCodeName(errObj.ExitCode), errObj.Retryable)
	}
	return exitError(errObj.ExitCode)
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
	return []string{"doctor", "backends", "models", "model", "route", "config", "capabilities"}
}

func unknownVerbError(verb string) envelope.Error {
	nearest, distance := nearestString(verb, rootVerbNames())
	var did string
	var nearestValue any
	var distanceValue any
	if distance <= 2 {
		did = "infer " + nearest
		nearestValue = nearest
		distanceValue = distance
	} else {
		did = "infer --help"
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
	did := verb + " --help"
	return envelope.Error{
		Code:       "E_UNKNOWN_FLAG",
		Message:    "unknown flag '" + given + "' for verb '" + verb + "'",
		DidYouMean: &did,
		ExitCode:   1,
		Retryable:  false,
		Details:    map[string]any{"verb": verb, "given": given, "nearest": nil, "distance": nil},
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
	case 0:
		return "success"
	case 1:
		return "user_input_error"
	case 3:
		return "tool_environment_error"
	case 4:
		return "runtime_retryable_error"
	default:
		return "unknown"
	}
}
