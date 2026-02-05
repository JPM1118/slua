package main

import (
	"os"

	"github.com/JPM1118/slua/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
