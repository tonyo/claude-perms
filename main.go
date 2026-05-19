package main

import (
	"os"

	"github.com/tonyo/claude-perms/cmd"
)

func main() {
	if err := cmd.NewRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}
