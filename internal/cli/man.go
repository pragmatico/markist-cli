package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

func newManCommand() *cobra.Command {
	var dir string

	cmd := &cobra.Command{
		Use:    "man",
		Short:  "Generate man pages",
		Long:   `Generates man pages for every markist command into --dir. Run by GoReleaser before packaging a release; not meant for interactive use.`,
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMan(cmd, dir)
		},
	}

	cmd.Flags().StringVar(&dir, "dir", ".", "output directory for generated man pages")

	return cmd
}

func runMan(cmd *cobra.Command, dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating %s: %w", dir, err)
	}

	header := &doc.GenManHeader{
		Title:   "MARKIST",
		Section: "1",
	}

	return doc.GenManTree(cmd.Root(), header, dir)
}
