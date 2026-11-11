# Copies TestH26ATellIsForTheSessionTold into a checkout of round 1
# (f3be27c), for collect.sh, with its check of the tell's Session field left
# out, since round 1 has no such field. What the test checks of behaviour is
# unchanged.
#   python3 -I round1.py <this commit's tell_test.go> <round 1's new test file>
import sys

src = open(sys.argv[1]).read()
body = src[src.index("// H26 round 2 (row 93)"):]
check = """	if e := r.snap(t).Entries[0]; e.Session != "s-one" {
		t.Fatalf("the tell names session %q, want s-one", e.Session)
	}
"""
assert body.count(check) == 1
body = body.replace(check, "")
open(sys.argv[2], "w").write('package main\n\nimport (\n\t"strings"\n\t"testing"\n)\n\n' + body)
