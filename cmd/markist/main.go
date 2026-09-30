// Command markist is the Markist CLI entry point. All command logic lives
// in internal/cli; main only wires the process exit code.
package main

import (
	"os"

	"github.com/pragmatico/markist-cli/internal/cli"
)

func main() {
	os.Exit(cli.Execute())
}
