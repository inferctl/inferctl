package main

import (
	_ "embed"
	"fmt"

	"github.com/spf13/cobra"
)

//go:embed robot_docs_guide.md
var robotDocsGuide string

type robotDocsGuideData struct {
	Name    string `json:"name"`
	Format  string `json:"format"`
	Source  string `json:"source"`
	Content string `json:"content"`
}

func newRobotDocsCommand(jsonFlag *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "robot-docs",
		Short: "Agent workflow documentation",
	}
	cmd.AddCommand(newRobotDocsGuideCommand(jsonFlag))
	return cmd
}

func newRobotDocsGuideCommand(jsonFlag *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "guide",
		Short: "Print the agent workflow guide",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			data := robotDocsGuideData{
				Name:    "inferctl Agent Guide",
				Format:  "markdown",
				Source:  "embedded:cmd/inferctl/robot_docs_guide.md",
				Content: robotDocsGuide,
			}
			return writeData(cmd, *jsonFlag, data, func() error {
				fmt.Fprint(cmd.OutOrStdout(), robotDocsGuide)
				return nil
			})
		},
	}
	return cmd
}
