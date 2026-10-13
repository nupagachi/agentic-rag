// Command app is the single entry point of the Agentic RAG backend. The first
// argument selects the role: "api" runs the HTTP API.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
)

const usageText = `Usage: app <command>

Commands:
  api    run the HTTP API (configured through environment variables)
  help   show this message
`

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := run(ctx, os.Args[1:], os.Getenv, os.Stdout, os.Stderr)
	stop()
	os.Exit(code)
}

// run dispatches the subcommand and returns the process exit code:
// 0 success, 1 runtime or configuration failure, 2 usage error.
func run(ctx context.Context, args []string, getenv func(string) string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		_, _ = fmt.Fprint(stderr, usageText)
		return 2
	}
	switch args[0] {
	case "api":
		return runAPI(ctx, args[1:], getenv, stdout, stderr)
	case "help", "-h", "--help":
		_, _ = fmt.Fprint(stdout, usageText)
		return 0
	default:
		_, _ = fmt.Fprintf(stderr, "app: unknown command %q\n\n%s", args[0], usageText)
		return 2
	}
}
