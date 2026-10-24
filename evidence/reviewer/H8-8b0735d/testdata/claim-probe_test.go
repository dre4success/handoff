package main

import (
    "testing"
    "time"
)

func TestReviewRestartedClaimResetsQuietClock(t *testing.T) {
    r := newRepo(t)
    r.setQuiet(t, "1s")
    path, sinkEnv := sink(t)
    req := r.request(t, "W1", r.commitA)
    first := r.run(t, "reviewer", "", sinkEnv, "claim", "--as", "reviewer", req)
    if first.code != 0 { t.Fatalf("first claim: %+v", first) }
    time.Sleep(1500 * time.Millisecond)
    restarted := r.run(t, "reviewer", "", sinkEnv, "claim", "--as", "reviewer", req)
    if restarted.code != 0 { t.Fatalf("restart claim: %+v", restarted) }
    t.Logf("restart claim stdout: %s; stderr: %s", restarted.out, restarted.err)
    if got := sentTo(t, path); len(got) != 0 {
        t.Fatalf("reviewer just reclaimed the request, but received overdue alerts: %q", got)
    }
}
