package main

import (
	"fmt"
	"os"

	"github.com/famarting/fws/internal/cli"
)

func main() {
	if err := cli.NewRoot().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "fws: "+err.Error())
		os.Exit(1)
	}
}
