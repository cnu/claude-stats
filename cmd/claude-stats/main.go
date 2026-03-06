package main

import (
	"os"

	"github.com/cnu/claude-stats/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
