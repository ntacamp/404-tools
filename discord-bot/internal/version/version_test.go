package version

import "testing"

func TestCommitURL(t *testing.T) {
	defer func(old string) { Commit = old }(Commit)

	Commit = "fdabadd7d58ad6ef40725e5099fb754a136997e6"
	if got, want := Get(), "fdabadd"; got != want {
		t.Errorf("Get() = %q, want %q", got, want)
	}
	if got, want := CommitURL(), "https://github.com/ntacamp/404-tools/commit/fdabadd"; got != want {
		t.Errorf("CommitURL() = %q, want %q", got, want)
	}
}

func TestCommitURLUnknown(t *testing.T) {
	defer func(old string) { Commit = old }(Commit)

	// Test binaries carry no VCS info, so with Commit unset this is a dev build.
	Commit = ""
	if got := CommitURL(); got != "" {
		t.Errorf("CommitURL() = %q, want empty for dev build", got)
	}
}
