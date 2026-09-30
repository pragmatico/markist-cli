package cli

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/pragmatico/markist-cli/internal/version"
)

func newVersionCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "version",
		Short:   "Print the markist version",
		Long:    `Prints the markist version, commit, build date, and the Go/OS/arch it was built for.`,
		Example: `  markist version`,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintf(cmd.OutOrStdout(), "markist %s (%s, %s) %s/%s\n",
				version.Version, version.Commit, version.Date, runtime.GOOS, runtime.GOARCH)
			return nil
		},
	}
	return cmd
}
