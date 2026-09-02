package main

import (
	"fmt"
	"io"
	"os"

	"github.com/kefyusuf/assurectl/internal/buildinfo"
)

const usage = `Usage: assurectl <command>

Commands:
  version    Print version information
  help       Show this help
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		_, _ = io.WriteString(stdout, usage)
		return 0
	}

	switch args[0] {
	case "help", "-h", "--help":
		_, _ = io.WriteString(stdout, usage)
		return 0
	case "version", "-v", "--version":
		_, _ = fmt.Fprintln(stdout, buildinfo.Summary())
		return 0
	default:
		_, _ = fmt.Fprintf(stderr, "unknown command: %s\n\n", args[0])
		_, _ = io.WriteString(stderr, usage)
		return 64
	}
}
