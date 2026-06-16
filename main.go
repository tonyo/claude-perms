package main

import (
	"os"
	"runtime/debug"

	"github.com/tonyo/claude-perms/cmd"
)

var version = "dev"

func main() {
	if version == "dev" {
		if info, ok := debug.ReadBuildInfo(); ok {
			if info.Main.Version != "" && info.Main.Version != "(devel)" {
				version = info.Main.Version
			} else {
				version = devVersion(info.Settings)
			}
		}
	}
	if err := cmd.NewRootCmd(version).Execute(); err != nil {
		os.Exit(1)
	}
}

func devVersion(settings []debug.BuildSetting) string {
	var commit, modified string
	for _, s := range settings {
		switch s.Key {
		case "vcs.revision":
			commit = s.Value
		case "vcs.modified":
			modified = s.Value
		}
	}
	if commit == "" {
		return "dev"
	}
	v := "dev+" + commit[:min(7, len(commit))]
	if modified == "true" {
		v += "-dirty"
	}
	return v
}
