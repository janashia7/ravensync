package main

import (
	"os"

	"github.com/ravensync/ravensync/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
