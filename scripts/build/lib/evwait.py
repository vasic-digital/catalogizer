#!/usr/bin/env python3
"""evwait.py - T005b slice: event-driven blocking primitives for the build dispatcher (no polling, no child process).

  evwait.py tail <file> <from_line> [<pid> <cmdline-substring>]
      Stream the lines of <file> after the first <from_line> lines to stdout, one per line, then block on inotify for more.
      With a writer <pid> (proven ours by /proc/<pid>/cmdline containing <cmdline-substring>) it exits 0 when the file is fully
      read and the writer is gone (watched through pidfd_open, so the death is an event, not a probe); a completed line does
      NOT end the stream while the writer lives, so a forged completed line in the stream cannot truncate it. With no usable
      <pid> (the build is finished, or its daemon is unknown) the first completed line ends the stream.
  evwait.py waitfile <path> [<timeout_seconds>]
      Block (inotify) until <path> exists. Exit 0 present, 3 timeout.
  evwait.py wait <build_dir> [<timeout_seconds>]
      Block until <build_dir>/terminal/callback.state is done or failed. Exit 0 done, 1 failed, 3 timeout (the only clock
      is the single blocking select timeout). Prints the terminal kind and callback state on stdout.
  evwait.py waitstate <file> <state,state,...> [<timeout_seconds>]
      Block (inotify on the file's directory) until the first line of <file> is one of the listed states. Exit 0 and print the state; 3 timeout.
  evwait.py serve <builds_root>
      The event hub's event source: prints one line `event` per batch of changes the hub must reconcile; the inotify watches stay armed between lines
      (an event that happens while the hub reconciles is queued, never lost): the builds root and each b-*/ directory (create/rename/delete of entries,
      NOT plain modification, so heartbeats appended to a journal wake nobody), each terminal/ and group/<id>/ directory (any change), and a pidfd per
      live pump proven by /proc (a pump that exits is an event). Ends when the root disappears.
All of them watch with inotify (ctypes) and re-evaluate their condition in-process on every event: while idle no process starts.
"""
import ctypes, ctypes.util, os, select, struct, sys, time

IN_MODIFY, IN_CLOSE_WRITE, IN_MOVED_TO, IN_CREATE = 0x2, 0x8, 0x80, 0x100
MASK = IN_MODIFY | IN_CLOSE_WRITE | IN_MOVED_TO | IN_CREATE
libc = ctypes.CDLL(ctypes.util.find_library("c") or "libc.so.6", use_errno=True)


def inotify():
    fd = libc.inotify_init1(os.O_NONBLOCK | os.O_CLOEXEC)
    if fd < 0:
        raise OSError(ctypes.get_errno(), "inotify_init1")
    return fd


def watch(fd, path):
    return libc.inotify_add_watch(fd, os.fsencode(path), MASK)


def drain(fd):
    try:
        while os.read(fd, 65536):
            pass
    except BlockingIOError:
        pass


def pid_is_ours(pid, needle):
    if pid <= 1:
        return False
    try:
        with open("/proc/%d/cmdline" % pid, "rb") as f:
            return needle.encode() in f.read()
    except OSError:
        return False


def cmd_tail(a):
    path, start = a[0], int(a[1])
    pid = int(a[2]) if len(a) > 2 else 0
    needle = a[3] if len(a) > 3 else ""
    ifd = inotify()
    d = os.path.dirname(os.path.abspath(path))
    watch(ifd, d)
    pfd = -1
    if pid > 1 and pid_is_ours(pid, needle):
        try:
            pfd = os.pidfd_open(pid)
        except OSError:
            pfd = -1
    alive = pfd >= 0
    no_pid = pfd < 0
    done_lines = 0
    buf = b""
    pos = 0
    out = sys.stdout.buffer
    while True:
        try:
            with open(path, "rb") as f:
                f.seek(pos)
                data = f.read()
        except OSError:
            data = b""
        if data:
            buf += data
            pos += len(data)
            while b"\n" in buf:
                line, buf = buf.split(b"\n", 1)
                done_lines += 1
                if done_lines <= start:
                    continue
                out.write(line + b"\n")
                out.flush()
                if no_pid and b'"kind":"completed"' in line:
                    return 0               # no writer to watch (the build is finished or its daemon is unknown): the completed line ends the stream
            continue                       # re-read before blocking: the file may have grown meanwhile
        if not alive:
            return 0                       # file fully read and no live writer
        r, _, _ = select.select([ifd, pfd], [], [])
        if pfd in r:
            alive = False                  # writer died: one more read pass, then exit
            os.close(pfd)
            pfd = -1
        if ifd in r:
            drain(ifd)


def cmd_wait(a):
    d = a[0]
    timeout = float(a[1]) if len(a) > 1 else None
    deadline = None if timeout is None else time.monotonic() + timeout
    ifd = inotify()
    watch(ifd, d)
    tw = None
    while True:
        if tw is None and os.path.isdir(os.path.join(d, "terminal")):
            tw = watch(ifd, os.path.join(d, "terminal"))
        try:
            with open(os.path.join(d, "terminal", "callback.state"), "rb") as f:
                st = f.read(64).decode().strip()
        except OSError:
            st = ""
        if st in ("done", "failed"):
            try:
                with open(os.path.join(d, "terminal", "state.json"), "rb") as f:
                    kind = f.read(4096).decode()
            except OSError:
                kind = "{}"
            print("callback_state=%s" % st)
            print(kind.strip())
            return 0 if st == "done" else 1
        left = None if deadline is None else deadline - time.monotonic()
        if left is not None and left <= 0:
            print("timeout")
            return 3
        r, _, _ = select.select([ifd], [], [], left)
        if r:
            drain(ifd)


def cmd_waitfile(a):
    path = a[0]
    timeout = float(a[1]) if len(a) > 1 else None
    deadline = None if timeout is None else time.monotonic() + timeout
    ifd = inotify()
    watch(ifd, os.path.dirname(os.path.abspath(path)))
    while not os.path.exists(path):
        left = None if deadline is None else deadline - time.monotonic()
        if left is not None and left <= 0:
            return 3
        r, _, _ = select.select([ifd], [], [], left)
        if r:
            drain(ifd)
    return 0


def cmd_waitstate(a):
    path, states = a[0], set(a[1].split(","))
    timeout = float(a[2]) if len(a) > 2 else None
    deadline = None if timeout is None else time.monotonic() + timeout
    ifd = inotify()
    watch(ifd, os.path.dirname(os.path.abspath(path)))
    while True:
        try:
            with open(path, "rb") as f:
                st = f.readline(64).decode().strip()
        except OSError:
            st = ""
        if st in states:
            print(st)
            return 0
        left = None if deadline is None else deadline - time.monotonic()
        if left is not None and left <= 0:
            print("timeout"); return 3
        r, _, _ = select.select([ifd], [], [], left)
        if r:
            drain(ifd)


IN_DELETE, IN_ISDIR = 0x200, 0x40000000
MASK_ENTRIES = IN_CREATE | IN_MOVED_TO | IN_DELETE
MASK_ALL = MASK | IN_DELETE


def pump_of(bdir):
    """(pid, alive) of the pump of an OPEN build directory (no terminal/, not queued), proven by /proc start time and cmdline; None otherwise."""
    if os.path.isdir(os.path.join(bdir, "terminal")) or os.path.exists(os.path.join(bdir, "queued")):
        return None
    try:
        pid, start = open(os.path.join(bdir, "pump.pid")).read().split()[:2]
        pid = int(pid)
    except (OSError, ValueError):
        return None
    if pid <= 1:
        return None
    try:
        st = open("/proc/%d/stat" % pid).read().rsplit(")", 1)[1].split()
        return pid, st[19] == start and b"_pump" in open("/proc/%d/cmdline" % pid, "rb").read()
    except (OSError, IndexError):
        return pid, False


def cmd_serve(a):
    """The event source of the hub: one line `event` on stdout per batch of changes the hub must reconcile; the watches stay armed between lines, so
    an event that happens while the hub reconciles is queued, never lost. Watched: the builds root and each b-*/ directory (create/rename/delete of
    entries, NOT plain modification: heartbeats appended to a journal wake nobody), each terminal/ and group/<id>/ directory (any change), and a pidfd
    per live pump (proven by /proc): a pump that exits is an event. Idle cost: one blocking select."""
    root = os.path.abspath(a[0])
    ifd = inotify()
    wd2path = {}

    def add(path, mask):
        wd = libc.inotify_add_watch(ifd, os.fsencode(path), mask)
        if wd >= 0:
            wd2path[wd] = path

    def arm(path):
        if not os.path.isdir(path):
            return
        base = os.path.basename(path)
        parent = os.path.basename(os.path.dirname(path))
        if path == root:
            add(path, MASK_ENTRIES)
            for n in sorted(os.listdir(path)):
                if n.startswith("b-") or n == "group":
                    arm(os.path.join(path, n))
        elif base.startswith("b-") and os.path.dirname(path) == root:
            add(path, MASK_ENTRIES)
            arm(os.path.join(path, "terminal"))
        elif base == "terminal":
            add(path, MASK_ALL)
        elif base == "group" and os.path.dirname(path) == root:
            add(path, MASK_ENTRIES)
            for g in sorted(os.listdir(path)):
                arm(os.path.join(path, g))
        elif parent == "group":
            add(path, MASK_ALL)
            arm(os.path.join(path, "terminal"))
    arm(root)
    pfds = {}                                   # pid -> pidfd
    out = sys.stdout

    def emit():
        out.write("event\n")
        out.flush()

    def scan():
        """register a pidfd per live pump; report a pump that is already gone (once)."""
        gone = False
        live = set()
        try:
            names = sorted(os.listdir(root))
        except OSError:
            return True
        for n in names:
            if not n.startswith("b-"):
                continue
            r = pump_of(os.path.join(root, n))
            if r is None:
                continue
            pid, alive = r
            if alive:
                live.add(pid)
                if pid not in pfds:
                    try:
                        pfds[pid] = os.pidfd_open(pid)
                    except OSError:
                        gone = True
            else:
                gone = True
        for pid in [p for p in pfds if p not in live]:
            os.close(pfds.pop(pid))
        return gone
    if scan():
        emit()
    emit()                                      # the first line is the hub's start-up reconcile
    while True:
        if not os.path.isdir(root):
            return 0
        r, _, _ = select.select([ifd] + list(pfds.values()), [], [])
        dead = [p for p, f in pfds.items() if f in r]
        for p in dead:
            os.close(pfds.pop(p))
        if ifd in r:
            try:
                buf = os.read(ifd, 65536)
            except BlockingIOError:
                buf = b""
            pos = 0
            while pos + 16 <= len(buf):
                wd, mask, _c, ln = struct.unpack_from("iIII", buf, pos)
                name = buf[pos + 16:pos + 16 + ln].split(b"\0", 1)[0].decode(errors="replace")
                pos += 16 + ln
                base = wd2path.get(wd)
                if base and (mask & (IN_CREATE | IN_MOVED_TO)) and (mask & IN_ISDIR):
                    arm(os.path.join(base, name))
        gone = scan()
        if dead or gone or ifd in r:
            emit()


if __name__ == "__main__":
    c = sys.argv[1] if len(sys.argv) > 1 else ""
    if c == "tail" and len(sys.argv) >= 4:
        sys.exit(cmd_tail(sys.argv[2:]))
    if c == "waitfile" and len(sys.argv) >= 3:
        sys.exit(cmd_waitfile(sys.argv[2:]))
    if c == "wait" and len(sys.argv) >= 3:
        sys.exit(cmd_wait(sys.argv[2:]))
    if c == "waitstate" and len(sys.argv) >= 4:
        sys.exit(cmd_waitstate(sys.argv[2:]))
    if c == "serve" and len(sys.argv) >= 3:
        sys.exit(cmd_serve(sys.argv[2:]))
    print("usage: evwait.py tail <file> <from_line> [pid needle] | wait <build_dir> [timeout]", file=sys.stderr)
    sys.exit(2)
