package main

import (
	"runtime/debug"

	"github.com/pentops/j5build/cmd/j5/internal/cli"
)

var Version = ""

func init() {
	if Version == "" {
		buildInfo, ok := debug.ReadBuildInfo()
		if !ok {
			Version = "local"
		} else {
			Version = buildInfo.Main.Version
		}
	}
}

func main() {
	cli.Version = Version
	cmdGroup := cli.CommandSet()
	cmdGroup.RunMain("j5", Version)
}
