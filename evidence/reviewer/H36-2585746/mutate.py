# Reviewer's mutation check for H36: break one thing in a scratch copy of the tree, to see that a test of row 107 fails for it.
# Usage: python3 mutate.py <tree> <a|c|d>
import sys,re
tree,which=sys.argv[1],sys.argv[2]
def sub(path,old,new,count=1):
    p=tree+'/'+path; s=open(p).read()
    assert old in s, (which, 'not found', old[:60])
    open(p,'w').write(s.replace(old,new,count))
if which=='a':   # round 6's /clear race: a hook that finds the other hook's binding answers "none"
    sub('cmd/handoff/session.go','''	if b, ok := a.readBinding(s.ID); ok {
		return settled(b, true) // another hook of s moved it meanwhile
	}
	if a.cut(s) {
		return binding{}, carriedBlocked
	}
	return binding{}, carriedNone
}''','''	if _, ok := a.readBinding(s.ID); ok {
		return binding{}, carriedNone
	}
	if a.cut(s) {
		return binding{}, carriedBlocked
	}
	return binding{}, carriedNone
}''')
    sub('cmd/handoff/session.go','''		if b, ok := a.readBinding(s.ID); ok {
			return settled(b, true)
		}
		return a.cutShort(s)''','''		return binding{}, carriedNone''')
elif which=='c': # no fence on a note's write
    s=open(tree+'/cmd/handoff/cmds.go').read()
    i=s.index('func (a *app) cmdNote'); j=s.index('if err := a.fenced(); err != nil {',i)
    s=s[:j]+'if err := error(nil); err != nil {'+s[j+len('if err := a.fenced(); err != nil {'):]
    open(tree+'/cmd/handoff/cmds.go','w').write(s)
elif which=='d': # no fence on a held role's delivery
    sub('cmd/handoff/session.go','if held && (!ok || b.Role != role || b.Gen != start.Gen) {','if false && held && (!ok || b.Role != role || b.Gen != start.Gen) {')
