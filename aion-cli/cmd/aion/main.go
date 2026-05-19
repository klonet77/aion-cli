package main

import (
	"fmt"
	"os"

	"github.com/klonet77/aion-cli/commands"
)

func main() {
	if err := commands.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
