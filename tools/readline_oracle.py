#!/usr/bin/env python3
"""Replay readline test cases in a real interactive bash and report mismatches.

Usage: python3 tools/readline_oracle.py internal/readline/testdata/cases.json

The initial line is set via a `bind -x` hook (READLINE_LINE/READLINE_POINT),
keys are sent as raw bytes, and a second hook prints the resulting line.
"""
import json, os, pty, re, select, shlex, sys, tempfile, time

CUR = "‸"

def key_bytes(k):
    if len(k) >= 2 and k[0] == "'" and k[-1] == "'":
        return k[1:-1].encode()
    named = {"esc": b"\x1b", "backspace": b"\x7f", "enter": b"\r", "alt+backspace": b"\x1b\x7f",
             "ctrl+_": b"\x1f", "left": b"\x1b[D", "right": b"\x1b[C",
             "up": b"\x1b[A", "down": b"\x1b[B", "home": b"\x1b[H", "end": b"\x1b[F",
             "delete": b"\x1b[3~"}
    if k in named:
        return named[k]
    if k.startswith("ctrl+") and len(k) == 6:
        return bytes([ord(k[5]) & 0x1F])
    if k.startswith("alt+") and len(k) == 5:
        return b"\x1b" + k[4].encode()
    raise ValueError(k)

def read_until(fd, pattern, timeout=3.0):
    buf = b""
    end = time.time() + timeout
    while time.time() < end:
        r, _, _ = select.select([fd], [], [], 0.05)
        if r:
            try:
                chunk = os.read(fd, 4096)
            except OSError:
                break
            buf += chunk
            m = re.search(pattern, buf, re.S)
            if m:
                return m
    raise TimeoutError(buf[-300:])

def run_case(tc):
    start = tc["start"]
    point = start.index(CUR)
    text = start.replace(CUR, "", 1)
    histfile = os.path.join(tempfile.gettempdir(), "termgame_oracle_history")
    with open(histfile, "w") as f:
        f.writelines(h + "\n" for h in tc.get("history") or [])
    pid, fd = pty.fork()
    if pid == 0:
        # HISTIGNORE keeps our setup commands out of history; HISTFILE seeds it.
        env = dict(os.environ, INPUTRC="/dev/null", PS1="$ ", HISTFILE=histfile,
                   HISTIGNORE="*", TERM="xterm")
        os.execvpe("bash", ["bash", "--norc", "--noprofile", "-i"], env)
    try:
        setup = [
            "stty -ixon",
            "bind 'set enable-bracketed-paste off'",
            "bind -x '\"\\C-o\": READLINE_LINE=$__L; READLINE_POINT=$__P'",
            "bind -x '\"\\C-^\": printf \"\\n<<%s|%s>>\\n\" \"$READLINE_POINT\" \"$READLINE_LINE\"'",
        ]
        setup.append("__L=" + shlex.quote(text) + "; __P=" + str(len(text[:point].encode())))
        setup.append("echo READY")
        for line in setup:
            os.write(fd, line.encode() + b"\r")
        read_until(fd, rb"\nREADY")
        time.sleep(0.05)
        os.write(fd, b"\x0f")  # ctrl+o: load start line
        time.sleep(0.05)
        for k in tc["keys"]:
            os.write(fd, key_bytes(k))
            time.sleep(0.03)
        os.write(fd, b"\x1e")  # ctrl+^: dump line
        m = read_until(fd, rb"<<(\d+)\|(.*?)>>")
        p, line = int(m.group(1)), m.group(2)
        return (line[:p] + CUR.encode() + line[p:]).decode()
    finally:
        os.kill(pid, 9)
        os.waitpid(pid, 0)
        os.close(fd)

def main():
    cases = json.load(open(sys.argv[1]))
    bad = 0
    for tc in cases:
        if tc.get("bash") is False:
            print(f"SKIP {tc['name']}")
            continue
        try:
            got = run_case(tc)
        except Exception as e:
            got = f"<error {e!r}>"
        ok = got == tc["want"]
        bad += not ok
        print(("ok   " if ok else "DIFF ") + tc["name"] + ("" if ok else f"\n     bash: {got!r}\n     want: {tc['want']!r}"))
    print(f"\n{len(cases)} cases, {bad} differ from bash")

main()
