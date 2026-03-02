package main

import (
	"os"

	"github.com/MH4GF/tq/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
