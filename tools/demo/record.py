#!/usr/bin/env python3
"""Drive bashgolf in a pty with scripted keys and record an asciicast v2 file.

Used to make assets/demo.gif:

    go build -o bashgolf .
    python3 tools/demo/record.py tools/demo/demo.keys demo.cast ./bashgolf
    agg --font-size 16 --idle-time-limit 3 --last-frame-duration 3 demo.cast assets/demo.gif

Each line of the keys file is "<delay-seconds> <key> [<key>...]", with keys
like ctrl+k, alt+f, enter, down or 'text'.
"""
import fcntl, json, os, pty, select, struct, sys, termios, time

COLS, ROWS = 110, 22
KEYS = {"enter": "\r", "esc": "\x1b", "up": "\x1b[A", "down": "\x1b[B",
        "left": "\x1b[D", "right": "\x1b[C", "space": " ", "tab": "\t",
        "backspace": "\x7f"}


def key(k):
    if k in KEYS:
        return KEYS[k]
    if k.startswith("ctrl+") and len(k) == 6:
        return chr(ord(k[5]) & 0x1f)
    if k.startswith("alt+"):
        return "\x1b" + k[4:]
    if k.startswith("'") and k.endswith("'"):
        return k[1:-1]
    raise SystemExit(f"unknown key {k}")


def main():
    script, out = sys.argv[1], sys.argv[2]
    steps = []
    for line in open(script):
        line = line.split("#", 1)[0].strip()
        if line:
            d, *ks = line.split(" ", 1)
            ks = ks[0].split() if ks else []
            # allow quoted text with spaces: join tokens between quotes
            merged, buf = [], None
            for t in ks:
                if buf is not None:
                    buf += " " + t
                    if t.endswith("'"):
                        merged.append(buf); buf = None
                elif t.startswith("'") and not (t.endswith("'") and len(t) > 1):
                    buf = t
                else:
                    merged.append(t)
            steps.append((float(d), merged))

    pid, fd = pty.fork()
    if pid == 0:
        os.environ.update(TERM="xterm-256color", COLORTERM="truecolor")
        os.execvp(sys.argv[3] if len(sys.argv) > 3 else "bashgolf",
                  [sys.argv[3] if len(sys.argv) > 3 else "bashgolf"])
    fcntl.ioctl(fd, termios.TIOCSWINSZ, struct.pack("HHHH", ROWS, COLS, 0, 0))

    events, t0 = [], time.monotonic()

    def pump(until):
        while True:
            left = until - time.monotonic()
            if left <= 0:
                return
            r, _, _ = select.select([fd], [], [], left)
            if r:
                try:
                    data = os.read(fd, 65536)
                except OSError:
                    return
                if not data:
                    return
                # answer terminal queries like a real terminal would
                if b"\x1b]11;?" in data:
                    os.write(fd, b"\x1b]11;rgb:2828/2a2a/3636\x1b\\")
                if b"\x1b]10;?" in data:
                    os.write(fd, b"\x1b]10;rgb:cccc/cccc/cccc\x1b\\")
                if b"\x1b[6n" in data:
                    os.write(fd, b"\x1b[1;1R")
                if b"\x1b[c" in data:
                    os.write(fd, b"\x1b[?62;22c")
                events.append([round(time.monotonic() - t0, 4), "o", data.decode("utf-8", "replace")])

    for delay, ks in steps:
        pump(time.monotonic() + delay)
        for k in ks:
            os.write(fd, key(k).encode())
            pump(time.monotonic() + 0.4 if len(ks) > 1 else time.monotonic())
    pump(time.monotonic() + 0.5)
    try:
        os.kill(pid, 9)
    except ProcessLookupError:
        pass

    with open(out, "w") as f:
        f.write(json.dumps({"version": 2, "width": COLS, "height": ROWS}) + "\n")
        for e in events:
            f.write(json.dumps(e) + "\n")


main()
