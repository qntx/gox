package main

import (
	"os"

	"github.com/qntx/gox/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
