package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/dre4success/handoff/internal/gitx"
)

// A session started outside the project (rows 101, 103). A CLI runs a
// session's hooks in the folder the session is in. Codex's session stays in
// the folder it started in; Claude Code's follows a cd, but each start and
// resume, /clear and /background put it back where this run of it started
// (evidence/h31/, the reviewer's evidence/reviewer/H31-c57d29e/). So the hooks
// of a session started outside the project don't reach it then: no brief, no
// new process noted for its role, and other agents are told no session has
// the role. The session can still take a role from inside the project, so
// handoff tells it, when it takes one or reads its brief, and it can tell the
// operator.

// warnOutside warns the session s, when this run of it started outside this
// project. transcript is the transcript a hook was given, or "".
func (a *app) warnOutside(w io.Writer, s session, transcript string) {
	dir, cli := a.startedOutside(s, transcript)
	if dir == "" {
		return
	}
	root := filepath.Dir(a.dir)
	name := filepath.Base(root)
	when, resume := "when it starts or resumes, and after /clear or /background", "claude --resume "+s.ID
	if cli == "codex" {
		when, resume = "always", "codex resume "+s.ID
	}
	cd := tilde(root)
	if strings.ContainsAny(root, " '\"$`\\!*?[]{}()&;|<>#~") {
		cd = shellQuote(root)
	}
	fmt.Fprintf(w, "handoff: warning: this session started in %s, outside %s. Its hooks run outside %s %s, so it can miss its brief and the work sent to it. Tell the operator. They can close this session and resume it from %s (`cd %s && %s`), which keeps this conversation, or start a session there and give it the role.\n\n",
		tilde(dir), name, name, when, name, cd, resume)
}

// startedOutside is the folder this run of s started in, and its CLI, if that
// folder is outside this project: in no git repository, or in one whose
// record is another. The folder is "" when the run started in the project,
// or when that can't be told.
func (a *app) startedOutside(s session, transcript string) (string, string) {
	dir, cli := runFolder(s, transcript)
	if !filepath.IsAbs(dir) {
		return "", cli
	}
	rec, err := gitx.RecordDir(dir)
	if err != nil {
		if gitx.NoRepository(err) {
			return dir, cli
		}
		return "", cli // git can't say: the folder is gone, or owned by someone else
	}
	if samePath(rec, a.dir) {
		return "", cli
	}
	return dir, cli
}

// runFolder is the folder this run of s started in, and its CLI. A transcript
// can't say: Claude Code and Codex each keep one for a session wherever it is
// resumed, and its first lines name where the conversation began (row 103).
//   - A Claude Code hook has CLAUDE_PROJECT_DIR, the folder the process
//     started or resumed in.
//   - A Claude Code command has no such variable, but its session's process,
//     CLAUDE_PID, stays in that folder while the session cds about, and so does
//     the worker a /background moves it to (evidence/h31/background.log).
//   - Codex writes a turn_context for each turn, naming the folder of the run;
//     the latest is in its rollout's last lines.
func runFolder(s session, transcript string) (string, string) {
	if d := os.Getenv("CLAUDE_PROJECT_DIR"); d != "" && s.PID > 0 {
		return d, "claude"
	}
	if s.PID > 0 {
		return processFolder(s.PID), "claude"
	}
	if sessionCLI() == "antigravity" {
		// It keeps no rollout, and its hooks take the agent's folder from their
		// input, wherever they run (row 102), so it isn't looked for (row 104).
		return "", "antigravity"
	}
	if transcript == "" || !rolloutOf(transcript, s.ID) {
		transcript = codexRollout(s.ID)
	}
	return lastTurnFolder(transcript), "codex"
}

// processFolder is the folder the process pid is in, as lsof, macOS's own,
// reports it; "" if it can't. It waits two seconds at most.
func processFolder(pid int) string {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	lsof := "/usr/sbin/lsof"
	if _, err := os.Stat(lsof); err != nil {
		lsof = "lsof"
	}
	out, err := exec.CommandContext(ctx, lsof, "-a", "-p", strconv.Itoa(pid), "-d", "cwd", "-Fn").Output()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		if dir, ok := strings.CutPrefix(line, "n"); ok {
			return dir
		}
	}
	return ""
}

// rolloutOf says whether path is named as Codex's rollout for id,
// rollout-<time>-<id>.jsonl.
func rolloutOf(path, id string) bool {
	base := filepath.Base(path)
	return strings.HasPrefix(base, "rollout-") && strings.HasSuffix(base, "-"+id+".jsonl")
}

// codexRollout finds Codex's rollout for the session id,
// <home>/sessions/<year>/<month>/<day>/rollout-<time>-<id>.jsonl. Of more than
// one, the latest written; "" if none. An id that isn't plain, which a
// binding takes by a hash of it, would be read as a pattern, so it finds none.
func codexRollout(id string) string {
	if !plainID.MatchString(id) {
		return ""
	}
	found, _ := filepath.Glob(filepath.Join(userConfig("codex"), "sessions", "*", "*", "*", "rollout-*-"+id+".jsonl"))
	latest, at := "", time.Time{}
	for _, p := range found {
		if st, err := os.Stat(p); err == nil && st.ModTime().After(at) {
			latest, at = p, st.ModTime()
		}
	}
	return latest
}

// lastTurnFolder is the folder a rollout's last turn_context names. A turn's
// lines can be long, so it reads the rollout's last 1 MiB, then, if need be,
// its last 16 MiB. Only a line whose own type is turn_context counts; one
// that holds such an object deeper, or a part line, doesn't. It never waits
// to open a file.
func lastTurnFolder(path string) string {
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return ""
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return ""
	}
	marker := []byte(`"turn_context"`) // with or without spaces after "type":
	for _, most := range []int64{1 << 20, 16 << 20} {
		off := max(st.Size()-most, 0)
		buf := make([]byte, st.Size()-off)
		if _, err := f.ReadAt(buf, off); err != nil && !errors.Is(err, io.EOF) {
			return ""
		}
		for end := len(buf); end > 0; {
			i := bytes.LastIndex(buf[:end], marker)
			if i < 0 {
				break
			}
			// A line that begins before what was read doesn't parse.
			start := bytes.LastIndexByte(buf[:i], '\n') + 1
			line := buf[start:]
			if n := bytes.IndexByte(line, '\n'); n >= 0 {
				line = line[:n]
			}
			var e struct {
				Type    string `json:"type"`
				Payload struct {
					Cwd string `json:"cwd"`
				} `json:"payload"`
			}
			if json.Unmarshal(line, &e) == nil && e.Type == "turn_context" && e.Payload.Cwd != "" {
				return e.Payload.Cwd
			}
			end = start
		}
	}
	return ""
}

// samePath says whether two paths name the same folder, through links.
func samePath(x, y string) bool {
	if rx, err := filepath.EvalSymlinks(x); err == nil {
		x = rx
	}
	if ry, err := filepath.EvalSymlinks(y); err == nil {
		y = ry
	}
	return filepath.Clean(x) == filepath.Clean(y)
}
