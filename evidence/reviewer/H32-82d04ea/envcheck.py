import os
n = sorted(k for k in os.environ if k.startswith("ANTIGRAVITY_"))
print("STDIN_TTY=%s STDOUT_TTY=%s CONVERSATION_ID=%s ANTIGRAVITY_NAMES=%d" % (os.isatty(0), os.isatty(1), "ANTIGRAVITY_CONVERSATION_ID" in n, len(n)))
