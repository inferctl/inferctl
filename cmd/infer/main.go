package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
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

func newRootCommand() *cobra.Command {
	var jsonFlag bool
	root := &cobra.Command{
		Use:           "infer",
		Short:         "Explain your local LLM stack",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.ErrOrStderr(), "infer: no verb specified")
			return exitError(1)
		},
	}
	root.PersistentFlags().BoolVar(&jsonFlag, "json", false, "emit JSON envelope")
	root.AddCommand(newCapabilitiesCommand(&jsonFlag))
	root.AddCommand(newConfigCommand(&jsonFlag))
	root.AddCommand(newBackendsCommand(&jsonFlag))
	root.AddCommand(newModelsCommand(&jsonFlag))
	root.AddCommand(newModelCommand(&jsonFlag))
	root.AddCommand(newDoctorCommand(&jsonFlag))
	root.AddCommand(newRouteCommand(&jsonFlag))
	return root
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
		fmt.Fprintln(cmd.ErrOrStderr(), errObj.Message)
		if errObj.DidYouMean != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "try: %s\n", *errObj.DidYouMean)
		}
	}
	return exitError(errObj.ExitCode)
}
