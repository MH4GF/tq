package cmd

import "runtime/debug"

func buildVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "dev"
	}
	var revision, time, modified string
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			revision = s.Value
		case "vcs.time":
			time = s.Value
		case "vcs.modified":
			modified = s.Value
		}
	}
	if time == "" {
		return "dev"
	}
	v := time
	if revision != "" {
		if len(revision) > 7 {
			revision = revision[:7]
		}
		v += " (" + revision + ")"
	}
	if modified == "true" {
		v += " dirty"
	}
	return v
}
