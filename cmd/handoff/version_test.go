package main

import (
	"os/exec"
	"regexp"
	"testing"
)

// H17: handoff version says which release is installed (proposal §17 rows
// 81, 82). Built in a clone, Go stamps the tag at a tag, and otherwise a
// pseudo-version such as v0.0.0-20261024210540-cbb09c235ffb; built in a
// linked worktree, (devel).
func TestH17VersionNamesTheBuild(t *testing.T) {
	want := regexp.MustCompile(`^handoff (\(devel\)|v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?(\+dirty)?)\n$`)
	for _, arg := range []string{"version", "--version"} {
		out, err := exec.Command(bin, arg).Output()
		if err != nil || !want.Match(out) {
			t.Errorf("handoff %s: %v, %q", arg, err, out)
		}
	}
}
