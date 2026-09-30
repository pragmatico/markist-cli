package cli

import "github.com/spf13/cobra"

// DocsURL is the CLI section of the Markist guide.
const DocsURL = "https://markist.xyz/guide#cli"

func newDocsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "docs",
		Short:   "Open the Markist CLI guide in your browser",
		Long:    `Opens https://markist.xyz/guide#cli, the CLI section of the Markist guide, in your default browser -- installation, authentication, and example commands.`,
		Example: `  markist docs`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return openBrowser(DocsURL)
		},
	}
	return cmd
}
