import re,sys
s=open(sys.argv[1],'rb').read().decode('utf-8','replace')
s=re.sub(r'\x1b\][^\x07\x1b]*(\x07|\x1b\\)','',s)
s=re.sub(r'\x1b\[[0-9;?<>=]*[ -/]*[@-~]','',s)
s=re.sub(r'\x1b[()][0-9A-Za-z]','',s); s=re.sub(r'\x1b[=>78]','',s)
s=s.replace('\r','\n')
lines=[l.rstrip() for l in s.split('\n')]
out=[];prev=None
for l in lines:
    if l and l!=prev: out.append(l)
    prev=l
print('\n'.join(out[-int(sys.argv[2]) if len(sys.argv)>2 else 0:]))
