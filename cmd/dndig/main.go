// Command dndig generates D&D campaign illustrations with Google's Gemini
// image models, with first-class character and scene continuity.
package main

import (
	"os"

	"github.com/mickume/dndig/internal/cli"
)

func main() {
	os.Exit(cli.Main(os.Args[1:], os.Stdout, os.Stderr))
}
