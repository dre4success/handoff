package gitx

import "testing"

// H18 (row 84): the git init hint is for git's two ways of saying it looked
// and found no repository, and for nothing else. The messages are git
// 2.55.0's, as the reviewer's probes recorded them
// (evidence/reviewer/H18-1c392de/probes.log); the mount-point one needs a
// volume of its own, so it is checked here, by its words.
func TestH18NoRepositoryMeansGitInitFixesIt(t *testing.T) {
	for msg, want := range map[string]bool{
		"fatal: not a git repository (or any of the parent directories): .git":                                                                               true,
		"fatal: not a git repository (or any parent up to mount point /Volumes)\nStopping at filesystem boundary (GIT_DISCOVERY_ACROSS_FILESYSTEM not set).": true,
		"fatal: not a git repository: (null)":                         false, // a worktree's .git file points at a missing path
		"fatal: not a git repository: '/tmp/nowhere'":                 false, // GIT_DIR names a missing path
		"fatal: detected dubious ownership in repository at '/tmp/x'": false,
	} {
		if got := noRepository(msg); got != want {
			t.Errorf("noRepository(%q) = %v, want %v", msg, got, want)
		}
	}
}
