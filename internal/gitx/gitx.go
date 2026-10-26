// Package gitx asks git questions. It only reads: no ref, index, working tree
// or object is written by anything here.
package gitx

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// RecordDirName is the record directory at the main worktree's root.
const RecordDirName = ".handoff"

func git(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	err := cmd.Run()
	if err != nil {
		msg := strings.TrimSpace(errb.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), msg)
	}
	return strings.TrimSpace(out.String()), nil
}

// RecordDir returns the one record directory for the repository that dir is
// in, whichever worktree or branch dir is on: .handoff/ beside the common git
// directory, at the main worktree's root.
func RecordDir(dir string) (string, error) {
	common, err := git(dir, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		// Only when git found no repository is git init the answer. Any other
		// failure, such as a repository owned by someone else, keeps git's
		// words alone (row 83).
		if noRepository(err.Error()) {
			return "", fmt.Errorf("not inside a git repository, which handoff needs; run `git init` first (%w)", err)
		}
		return "", fmt.Errorf("not inside a git repository: %w", err)
	}
	if filepath.Base(common) != ".git" {
		return "", fmt.Errorf("the common git directory %s is not a .git directory inside a worktree; handoff needs a non-bare repository", common)
	}
	return filepath.Join(filepath.Dir(common), RecordDirName), nil
}

// noRepository says whether git's words mean it looked for a repository and
// found none, which git init fixes: "not a git repository (or any of the
// parent directories)", or "(or any parent up to mount point …)" at a
// filesystem boundary. A pointer to a repository that isn't there, a
// worktree's .git file or GIT_DIR, reads "not a git repository: <path>", and
// git init does not fix it (row 84).
func noRepository(msg string) bool {
	return strings.Contains(msg, "not a git repository (or any")
}

// ResolveCommit turns a ref into a full commit id, or fails if it names no
// commit.
func ResolveCommit(dir, ref string) (string, error) {
	if ref == "" {
		return "", errors.New("no commit given")
	}
	sha, err := git(dir, "rev-parse", "--verify", "--quiet", "--end-of-options", ref+"^{commit}")
	if err != nil {
		return "", fmt.Errorf("%q does not name a commit in this repository", ref)
	}
	return sha, nil
}

// MergeBase returns the best common ancestor of two commits.
func MergeBase(dir, a, b string) (string, error) {
	sha, err := git(dir, "merge-base", "--end-of-options", a, b)
	if err != nil || sha == "" {
		return "", fmt.Errorf("%.12s and %s have no merge base; pass --base", a, b)
	}
	return sha, nil
}

// IsAncestor reports whether commit a is reachable from commit b.
func IsAncestor(dir, a, b string) (bool, error) {
	cmd := exec.Command("git", "merge-base", "--is-ancestor", "--end-of-options", a, b)
	cmd.Dir = dir
	err := cmd.Run()
	var exit *exec.ExitError
	switch {
	case err == nil:
		return true, nil
	case errors.As(err, &exit) && exit.ExitCode() == 1:
		return false, nil
	default:
		return false, fmt.Errorf("git merge-base --is-ancestor %.12s %.12s: %w", a, b, err)
	}
}

// Parents returns a commit's parents, in order.
func Parents(dir, commit string) ([]string, error) {
	out, err := git(dir, "rev-list", "--parents", "-n", "1", "--end-of-options", commit)
	if err != nil {
		return nil, err
	}
	f := strings.Fields(out)
	if len(f) == 0 {
		return nil, fmt.Errorf("no such commit %.12s", commit)
	}
	return f[1:], nil
}

// TopLevel returns the root of the worktree that dir is in.
func TopLevel(dir string) (string, error) {
	return git(dir, "rev-parse", "--show-toplevel")
}

// MainBranch guesses the main branch of the repository whose main worktree
// is dir: main, else master, else the branch that worktree is on, even one
// with no commit yet.
func MainBranch(dir string) (string, error) {
	for _, b := range []string{"main", "master"} {
		if _, err := git(dir, "rev-parse", "--verify", "--quiet", "refs/heads/"+b); err == nil {
			return b, nil
		}
	}
	b, err := git(dir, "symbolic-ref", "--quiet", "--short", "HEAD")
	if err != nil || b == "" {
		return "", errors.New("no main or master branch, and the main worktree is on no branch; name the main branch with --main")
	}
	return b, nil
}
