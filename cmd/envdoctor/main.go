// Command envdoctor diagnoses development environment problems.
//
// M1 scope: collect platform, shell and PATH facts, print them, and render an
// empty rule set. No rule exists yet, so `diagnose` never reports a finding and
// never modifies anything.
package main

import (
	"os"

	"github.com/shellghostt/envdoctor/internal/cli"
)

// version is overridden at build time with:
//
//	go build -ldflags "-X main.version=$(git describe --tags --always)"
var version = "dev"

func main() {
	os.Exit(cli.Main(os.Args, cli.App{
		Version: version,
		Stdout:  os.Stdout,
		Stderr:  os.Stderr,
		Getenv:  os.Getenv,
	}))
}
