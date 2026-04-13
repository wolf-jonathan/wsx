package main

import (
	"fmt"
	"os"

	"github.com/wolf-jonathan/workspace-x/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
