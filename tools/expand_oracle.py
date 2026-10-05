#!/usr/bin/env python3
"""Ask real bash for the history expansion of each case and write the
expectations used by the Go tests.

Usage: python3 tools/expand_oracle.py internal/readline/testdata/expand_inputs.json \
           > internal/readline/testdata/expand.json

Each input is {"history": [...], "line": "..."}; bash's `history -p` gives
the expansion, or fails for a bad event.
"""
import json, shlex, subprocess, sys

def expand(history, line):
    script = "set -o history; history -c; "
    script += "".join("history -s %s; " % shlex.quote(h) for h in history)
    script += "history -p %s" % shlex.quote(line)
    r = subprocess.run(["bash", "--norc", "--noprofile", "-c", script],
                       capture_output=True, text=True)
    if r.returncode != 0:
        return None
    return r.stdout.rstrip("\n")

cases = json.load(open(sys.argv[1]))
out = []
for c in cases:
    got = expand(c["history"], c["line"])
    d = dict(c)
    if got is None:
        d["error"] = True
    else:
        d["want"] = got
    out.append(d)
print("[\n" + ",\n".join("  " + json.dumps(d, ensure_ascii=False) for d in out) + "\n]")
