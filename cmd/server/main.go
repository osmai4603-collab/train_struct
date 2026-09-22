// Command server is the train service entry point. It bootstraps the
// application (configuration, logger, Postgres, routing) and runs the HTTP
// server through its full lifecycle until SIGINT/SIGTERM triggers a graceful
// drain and shutdown.
package main

import (
	"fmt"
	"os"

	"train/internal/platform/app"
)

func main() {
	a := app.New()

	if err := a.Bootstrap(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "bootstrap failed: %v\n", err)
		os.Exit(1)
	}

	if err := a.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "application failed: %v\n", err)
		os.Exit(1)
	}
}
