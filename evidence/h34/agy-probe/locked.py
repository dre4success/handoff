import fcntl,os,sys,errno
try: f=open(sys.argv[1])
except FileNotFoundError: print("no-lockfile"); sys.exit(1)
try:
    fcntl.flock(f, fcntl.LOCK_SH|fcntl.LOCK_NB); fcntl.flock(f, fcntl.LOCK_UN); print("free"); sys.exit(1)
except OSError as e:
    print("held"); sys.exit(0)
