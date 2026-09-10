// Command the-line creates tracks, optimizes lines and renders animations.
package main

import (
	"fmt"
	"os"

	"github.com/TheFellow/the-line/internal/cli"
)

func main() {
	if err := cli.Run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "the-line:", err)
		os.Exit(1)
	}
}
