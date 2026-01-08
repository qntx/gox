package main

import (
	"os"

	"github.com/qntx/gox/internal/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
