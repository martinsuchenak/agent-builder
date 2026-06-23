// Command agent-builder compiles canonical agent-builder artifacts to platform-specific formats.
package main

import (
	"os"

	"agent-builder/internal/cli"
)

func main() {
	os.Exit(cli.Main(os.Args[1:]))
}
