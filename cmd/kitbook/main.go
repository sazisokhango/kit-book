// Command kitbook is the equipment-booking CLI for Ridgeline Mountain Rescue.
package main

import (
	"os"

	"bitbucket.org/psybergate/kitbook/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
