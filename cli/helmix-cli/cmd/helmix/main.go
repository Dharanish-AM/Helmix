package main

import (
	"fmt"
	"os"

	"github.com/your-org/helmix/cli/helmix-cli/cmd/helmix/root"
)

func main() {
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}
