package main

import (
	"fmt"
	"os"

	"github.com/fury-labs/fury-bridge/cmd/fury-relayer/cmd"
)

func main() {
	rootCmd, err := cmd.NewRootCmd()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
