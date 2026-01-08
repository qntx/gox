package main

import (
	"os"

	"gox.qntx.fun/internal/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
