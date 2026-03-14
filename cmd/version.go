package cmd

import "runtime/debug"

var version = "0.18.0"

func buildVersion() string {
	v := "v" + version
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, s := range info.Settings {
			if s.Key == "vcs.modified" && s.Value == "true" {
				v += " (dirty)"
				break
			}
		}
	}
	return v
}
