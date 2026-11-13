// Package version reports which build of the bot is running.
package version

import "runtime/debug"

// Commit is the git SHA the binary was built from. Release builds set it via
//
//	-ldflags "-X github.com/ntacamp/404-tools/discord-bot/internal/version.Commit=<sha>"
//
// (the Docker image does this from the GIT_SHA build arg). When unset, Get
// falls back to the VCS info Go embeds on a plain `go build` inside a git
// checkout.
var Commit = ""

// Get returns the short commit SHA, suffixed with "-dirty" for builds from a
// modified working tree, or "dev" if no version information is available.
func Get() string {
	if Commit != "" {
		return short(Commit)
	}

	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "dev"
	}
	var rev string
	var dirty bool
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value
		case "vcs.modified":
			dirty = s.Value == "true"
		}
	}
	if rev == "" {
		return "dev"
	}
	if dirty {
		return short(rev) + "-dirty"
	}
	return short(rev)
}

func short(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}
