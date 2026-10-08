#!/usr/bin/env python3
"""compose_scan.py - the structural scanner of the three compose files of the test-infrastructure stack (WF17 fix round 5, TI-G1; constitution 11.4.201(7)(a), 11.4.245, 11.4.252).

Why it exists: the previous scanner was a DENY-LIST over per-leaf strings (credential-looking key names, URL userinfo, `--requirepass` flags, ...). Thirty-one same-class shapes bypassed it (a literal in a
`${X:-default}`, a credential under a key name it did not know, an exec-form argument, a label, `build.args`, an `env_file`, a service outside its `want` set, `devices`/`pid`/`userns_mode`, a bind of `/` or of
the podman socket, ...), and docker-compose.test-infra.nfs.yml was scanned by nothing. This scanner READS STRUCTURE and ALLOW-LISTS:

  1. RENDER each file with the REAL `podman-compose config`, in an EMPTY environment, with a sentinel env file (every TI_* variable the file references = a unique sentinel; ports = unique numbers).
     What a literal would be is then decidable: a value that is not made of sentinels / container references (`$VAR`) is a literal.
  2. RAW checks on the source YAML: every `${...}` must be a `${TI_NAME}` / `${TI_NAME:?message}` reference; any other variable, any default form `${X:-lit}` / `${X-lit}` / `${X:+..}` is a finding (the two image
     placeholders TI_NFS_IMAGE and TI_MINIO_IMAGE may default to a reference that cannot resolve: they hold no credential).
  3. ALLOW-LIST per service over the rendered tree: the set of keys, the exact environment (name -> template), the exact command / entrypoint / healthcheck test, the exact labels, the exact ports (loopback,
     sentinel host port, fixed container port), the exact volumes (sub-directories of the sentinel data dir), cap_add, image (digest-pinned and equal to the lock entry), `pull_policy: never`; the exact
     service SET of each file; the project network name and labels; `x-podman: in_pod: false`. Anything outside the allow-list is a finding. The declared builder (host network) and emulator (privileged,
     /dev/kvm, fixed ports) of docker-compose.build.yml are the only exceptions, and their credential-bearing environment keys must still be pure references.
  A finding NEVER prints the offending value (it may be a credential): it names the location and a length and an 8-hex digest.

Usage:  compose_scan.py scan <infra|nfs|build> [--root DIR]            prints one finding per line; exit 0 clean, 1 findings, 2 usage / cannot render
        compose_scan.py mutants <mutants.py> [--root DIR] [--only a,b]  applies every mutant of the module to a scratch copy of the real files, scans, prints `MUTANT <name> CAUGHT|SURVIVED <first finding>`;
                                                                        exit 1 when a mutant that must be caught survived or an identity mutant produced a finding
"""
import hashlib
import os
import re
import shutil
import subprocess
import sys
import tempfile

import yaml

SERVICE_KEYS = {"image", "pull_policy", "labels", "environment", "command", "healthcheck", "ports", "tmpfs", "volumes", "networks", "mem_limit", "cpus", "cap_add", "deploy", "profiles", "depends_on"}
BUILDER_EXTRA = {"build", "network_mode"}
EMULATOR_EXTRA = {"privileged", "devices"}
IMAGE_DEFAULT_VARS = {"TI_NFS_IMAGE", "TI_MINIO_IMAGE"}
# the builder's own non-secret switches: container-build.sh exports them and the compose file defaults them (the one place a non-TI variable and a default form are legitimate)
BUILDER_VARS = {"BUILD_VERSION", "BUILD_NUMBER", "BUILD_COMPONENTS", "FORCE_BUILD", "SKIP_TESTS", "SKIP_EMULATOR_TESTS", "SKIP_E2E_TESTS", "SKIP_SECURITY_TESTS"}
HISTORICAL = ("testpass", "testuser", "minioadmin", "catalogizer_test_pass", "catalogizer_test", "test123")
CREDKEY = re.compile(r"(AUTH|PASS|PASSWD|PASSWORD|SECRET|TOKEN|KEY|CRED|USER)", re.I)


def S(name):
    """The sentinel a variable renders as."""
    return "S3NT_" + name


PORTS = {}


def env_file_for(text_by_file):
    """Sentinel env: every TI_* variable referenced by any of the files."""
    names = []
    for t in text_by_file:
        for m in re.finditer(r"\$\{(TI_[A-Z0-9_]+)", t):
            if m.group(1) not in names:
                names.append(m.group(1))
    lines, n = [], 41100
    for v in names:
        if v.startswith("TI_PORT_"):
            n += 1
            PORTS[v] = n
            lines.append("%s=%d" % (v, n))
        elif v == "TI_NFS_IMAGE":
            lines.append("%s=localhost/s3nt-nfs@sha256:%s" % (v, "a" * 64))
        elif v == "TI_MINIO_IMAGE":
            lines.append("%s=localhost/s3nt-minio:none" % v)
        elif v == "TI_DATA_DIR":
            lines.append("%s=/s3nt/data" % v)
        else:
            lines.append("%s=%s" % (v, S(v)))
    return "\n".join(lines) + "\n"


def mask(v):
    s = str(v)
    return "len=%d sha8=%s" % (len(s), hashlib.sha256(s.encode()).hexdigest()[:8])


def tpl(s):
    """Expand {TI_X} placeholders of an expected template into their sentinels."""
    def one(m):
        n = m.group(1)
        if n.startswith("TI_PORT_"):
            return str(PORTS.get(n, "?"))
        if n == "TI_DATA_DIR":
            return "/s3nt/data"
        return S(n)
    return re.sub(r"\{(TI_[A-Z0-9_]+)\}", one, s)


LABELS = {"project": "catalogizer", "op_id": "{TI_OP_ID}", "catalogizer.op_id": "{TI_OP_ID}", "catalogizer.test_project": "{TI_PROJECT}", "catalogizer.test_root": "{TI_ROOT_HASH}"}
PG_ENV = {"POSTGRES_DB": "{TI_POSTGRES_DB}", "POSTGRES_USER": "{TI_POSTGRES_USER}", "POSTGRES_PASSWORD": "{TI_POSTGRES_PASSWORD}"}
PG_HC = ["CMD-SHELL", 'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -tAc \'SELECT 1\' | grep -qx 1']
REDIS_ENV = {"REDIS_PASSWORD": "{TI_REDIS_PASSWORD}", "REDISCLI_AUTH": "{TI_REDIS_PASSWORD}"}
REDIS_CMD = ["sh", "-c", "exec redis-server --save '' --appendonly no --requirepass \"$REDIS_PASSWORD\""]
REDIS_HC = ["CMD-SHELL", "redis-cli ping | grep -qx PONG"]

# name -> allow-list of the owned services (the security-relevant fields; timing, limits and tmpfs are free)
INFRA = {
    "postgres": dict(image="IMG-INFRA-POSTGRES", env=PG_ENV, command=None, hc=PG_HC, ports=[("TI_PORT_POSTGRES", 5432)], volumes=[], cap=[], nets=["test-network"]),
    "redis": dict(image="IMG-INFRA-REDIS", env=REDIS_ENV, command=REDIS_CMD, hc=REDIS_HC, ports=[("TI_PORT_REDIS", 6379)], volumes=[], cap=[], nets=["test-network"]),
    "ftp": dict(image="IMG-INFRA-FTP", env={"FTP_USER_NAME": "{TI_FTP_USER}", "FTP_USER_PASS": "{TI_FTP_PASSWORD}", "FTP_USER_HOME": "/home/ftpusers/ftpdata", "FTP_PASSIVE_PORTS": "30000:30009",
                                            "FTP_MAX_CLIENTS": "10", "FTP_MAX_CONNECTIONS": "5"}, command=None, hc=None, ports=[("TI_PORT_FTP", 21)], volumes=["{TI_DATA_DIR}/ftp:/home/ftpusers/ftpdata"], cap=["AUDIT_WRITE"], nets=["test-network"]),
    "smb": dict(image="IMG-INFRA-SMB", env={"USER": "{TI_SMB_USER};{TI_SMB_PASSWORD}", "SHARE": "testshare;/test-data;yes;no;no;{TI_SMB_USER};{TI_SMB_USER};{TI_SMB_USER};Catalogizer test share",
                                            "GLOBAL": "server min protocol = SMB2", "PERMISSIONS": "yes", "NMBD": "no"}, command=None, hc=None, ports=[("TI_PORT_SMB", 445)], volumes=["{TI_DATA_DIR}/smb:/test-data"], cap=[], nets=["test-network"]),
    "webdav": dict(image="IMG-INFRA-WEBDAV", env={"AUTH_TYPE": "Basic", "USERNAME": "{TI_WEBDAV_USER}", "PASSWORD": "{TI_WEBDAV_PASSWORD}", "LOCATION": "/"}, command=None, hc=None,
                   ports=[("TI_PORT_WEBDAV", 80)], volumes=["{TI_DATA_DIR}/dav:/var/lib/dav/data"], cap=[], nets=["test-network"]),
    "minio": dict(image=None, env={"MINIO_ROOT_USER": "{TI_MINIO_ROOT_USER}", "MINIO_ROOT_PASSWORD": "{TI_MINIO_ROOT_PASSWORD}"}, command=["server", "/data"], hc=None, ports=[], volumes=[], cap=[], nets=["test-network"], profiles=["minio-blocked"]),
}
NFS = {"nfs": dict(image=None, env={}, command=None, hc=None, ports=[], volumes=[], cap=[], nets=["test-network"])}
BUILD = {
    "postgres": dict(image="IMG-INFRA-POSTGRES", env=PG_ENV, command=None, hc=PG_HC, ports=[("TI_PORT_POSTGRES", 5432)], volumes=[], cap=[], nets=None),
    "redis": dict(image="IMG-INFRA-REDIS", env=REDIS_ENV, command=REDIS_CMD, hc=REDIS_HC, ports=[("TI_PORT_REDIS", 6379)], volumes=[], cap=[], nets=None),
}
BUILD_DECLARED = {"catalogizer-builder": BUILDER_EXTRA, "android-emulator": EMULATOR_EXTRA}
KINDS = {"infra": (INFRA, ["docker-compose.test-infra.yml"], ["--profile", "minio-blocked"]),
         "nfs": (NFS, ["docker-compose.test-infra.yml", "docker-compose.test-infra.nfs.yml"], []),
         "build": (BUILD, ["docker-compose.build.yml"], ["--profile", "emulator"])}


def raw_leaves(o, path=()):
    if isinstance(o, dict):
        for k, v in o.items():
            yield from raw_leaves(v, path + (str(k),))
    elif isinstance(o, (list, tuple)):
        for i, v in enumerate(o):
            yield from raw_leaves(v, path + (i,))
    elif isinstance(o, str):
        yield path, o
    elif o is not None:
        yield path, str(o)


def raw_checks(kind, root, files, out):
    for f in files:
        src = yaml.safe_load(open(os.path.join(root, f)).read()) or {}
        if (src.get("x-podman") or {}).get("in_pod") is not False:
            out.append("in_pod_not_disabled_in_file %s (each compose file carries x-podman.in_pod: false itself: podman-compose 1.5.0 defaults it to true)" % f)
        for path, leaf in raw_leaves(src):
            for m in re.finditer(r"\$\{([^}]*)\}", leaf):
                body = m.group(1)
                nm = re.match(r"^([A-Za-z_][A-Za-z0-9_]*)(.*)$", body)
                where = "%s:%s" % (f, ".".join(str(p) for p in path))
                if not nm:
                    out.append("unparsable_variable %s" % where); continue
                name, rest = nm.group(1), nm.group(2)
                if name in BUILDER_VARS and tuple(path[:3]) == ("services", "catalogizer-builder", "environment"):
                    continue
                if rest == "" or rest.startswith(":?") or rest.startswith("?"):
                    if not name.startswith("TI_"):
                        out.append("non_ti_variable %s (%s)" % (where, name))
                    continue
                if name in IMAGE_DEFAULT_VARS and path and path[-1] == "image" and (rest.startswith(":-") or rest.startswith("-")):
                    # the placeholder must be a reference that CANNOT resolve (so an unset variable never pulls or runs something else)
                    if not re.fullmatch(r"(:-|-)localhost/blocked-[a-z0-9-]+:none", rest):
                        out.append("image_placeholder_can_resolve %s" % where)
                    continue
                out.append("default_or_substitution_form %s (%s%s...)" % (where, name, rest[:2]))
            for h in HISTORICAL:
                if h in leaf:
                    out.append("historical_literal %s contains %s" % (":".join([f, ".".join(str(p) for p in path)]), h))


def render(kind, root, text_overrides, out):
    spec, files, extra = KINDS[kind]
    tmp = tempfile.mkdtemp(prefix="composescan.")
    try:
        paths = []
        for f in files:
            t = text_overrides.get(f)
            if t is None:
                t = open(os.path.join(root, f)).read()
            p = os.path.join(tmp, f)
            open(p, "w").write(t)
            paths.append(p)
        envp = os.path.join(tmp, "sent.env")
        open(envp, "w").write(env_file_for([open(p).read() for p in paths]))
        cmd = ["podman-compose"] + extra + ["--env-file", envp]
        for p in paths:
            cmd += ["-f", p]
        cmd.append("config")
        e = {"PATH": os.environ.get("PATH", ""), "HOME": os.environ.get("HOME", "/"), "XDG_RUNTIME_DIR": os.environ.get("XDG_RUNTIME_DIR", "")}
        r = subprocess.run(cmd, env=e, cwd=tmp, capture_output=True, text=True)
        if r.returncode != 0:
            out.append("render_failed %s: %s" % (kind, (r.stderr or r.stdout).strip().splitlines()[-1][:160] if (r.stderr or r.stdout).strip() else "rc=%d" % r.returncode))
            return None
        return yaml.safe_load(r.stdout), paths
    finally:
        shutil.rmtree(tmp, ignore_errors=True)


def as_dict(v):
    if isinstance(v, list):
        return dict(x.split("=", 1) if "=" in x else (x, "") for x in v)
    return v or {}


def lock_digest(root, image_id):
    d = yaml.safe_load(open(os.path.join(root, "build/containers/images.lock.yaml")))
    return [i["digest"] for i in d["images"] if i["id"] == image_id][0]


def pure_ref(v):
    """True when a rendered value is made only of sentinels and container references."""
    s = re.sub(r"S3NT_[A-Z0-9_]+", "", str(v))
    s = re.sub(r"\$\{?[A-Za-z_][A-Za-z0-9_]*\}?", "", s)
    return re.fullmatch(r"[\s;:,/]*", s) is not None


def expect(out, where, got, want):
    if got != want:
        out.append("not_allowlisted %s (%s)" % (where, mask(got)))


def check_service(kind, name, svc, spec, root, out, declared):
    where = "%s.%s" % (kind, name)
    allowed = set(SERVICE_KEYS) | (declared or set())
    for k in svc:
        if k not in allowed:
            out.append("key_not_allowed %s.%s" % (where, k))
    if declared is not None:
        # the declared builder / emulator: no allow-list, but a credential-bearing environment key must be a pure reference and nothing historical may remain
        env = as_dict(svc.get("environment"))
        for k, v in env.items():
            if CREDKEY.search(k) and not pure_ref(v):
                out.append("literal_credential %s.environment.%s (%s)" % (where, k, mask(v)))
        bargs = (svc.get("build") or {}).get("args") if isinstance(svc.get("build"), dict) else None
        for k, v in as_dict(bargs).items():
            if CREDKEY.search(str(k)) and not pure_ref(v):
                out.append("literal_credential %s.build.args.%s (%s)" % (where, k, mask(v)))
        return
    s = spec[name]
    # image: digest-pinned and equal to the lock entry (the two local placeholders are pinned by a sentinel digest or a cannot-resolve tag)
    img = str(svc.get("image", ""))
    if s["image"]:
        m = re.search(r"@(sha256:[0-9a-f]{64})$", img)
        if not m:
            out.append("image_not_digest_pinned %s (%s)" % (where, mask(img)))
        elif m.group(1) != lock_digest(root, s["image"]):
            out.append("image_digest_not_in_lock %s" % where)
    elif name == "nfs":
        if not re.search(r"@sha256:[0-9a-f]{64}$", img):
            out.append("image_not_digest_pinned %s (%s)" % (where, mask(img)))
    if svc.get("pull_policy") != "never" and name != "minio":
        out.append("pull_policy_not_never %s" % where)
    # labels, exactly
    labs = as_dict(svc.get("labels"))
    expect(out, where + ".labels", dict(labs), {k: tpl(v) for k, v in LABELS.items()})
    # environment, exactly (a literal that is not on the allow-list is a finding whatever its key is called)
    env = {str(k): ("" if v is None else str(v)) for k, v in as_dict(svc.get("environment")).items()}
    want_env = {k: tpl(v) for k, v in s["env"].items()}
    for k in sorted(set(env) | set(want_env)):
        if env.get(k) != want_env.get(k):
            out.append("env_not_allowlisted %s.environment.%s (%s)" % (where, k, mask(env.get(k, "<absent>"))))
    # command / entrypoint / healthcheck
    expect(out, where + ".command", svc.get("command"), s["command"])
    hc = svc.get("healthcheck")
    expect(out, where + ".healthcheck.test", (hc or {}).get("test") if isinstance(hc, dict) else hc, s["hc"])
    # ports: loopback, the sentinel host port of THIS service, the fixed container port
    got_ports = [str(p) for p in (svc.get("ports") or [])]
    want_ports = ["127.0.0.1:%d:%d" % (PORTS[v], c) for v, c in s["ports"]]
    expect(out, where + ".ports", got_ports, want_ports)
    expect(out, where + ".volumes", [str(v) for v in (svc.get("volumes") or [])], [tpl(v) for v in s["volumes"]])
    expect(out, where + ".cap_add", list(svc.get("cap_add") or []), s["cap"])
    nets = svc.get("networks")
    expect(out, where + ".networks", (list(nets) if isinstance(nets, (list, dict)) else nets), s["nets"])
    if "profiles" in s:
        expect(out, where + ".profiles", list(svc.get("profiles") or []), s["profiles"])


def scan(kind, root, text_overrides=None):
    text_overrides = text_overrides or {}
    out = []
    spec, files, _ = KINDS[kind]
    # raw checks run on the (possibly mutated) source
    tmp = tempfile.mkdtemp(prefix="composeraw.")
    try:
        for f in files:
            t = text_overrides.get(f)
            if t is None:
                t = open(os.path.join(root, f)).read()
            os.makedirs(os.path.dirname(os.path.join(tmp, f)), exist_ok=True)
            open(os.path.join(tmp, f), "w").write(t)
        try:
            raw_checks(kind, tmp, files, out)
        except yaml.YAMLError as e:
            out.append("yaml_unparsable %s" % str(e).splitlines()[0][:100])
            return out
    finally:
        shutil.rmtree(tmp, ignore_errors=True)
    r = render(kind, root, text_overrides, out)
    if r is None:
        return out
    tree, _ = r
    services = tree.get("services") or {}
    if kind == "nfs":
        names = {"nfs"}
        if "nfs" not in services:
            out.append("service_missing nfs")
        for n in services:
            if n in INFRA:
                continue   # the infra services come from the first file: scanned as kind infra
            if n != "nfs":
                out.append("unknown_service %s" % n)
        for n in ("nfs",):
            if n in services:
                check_service(kind, n, services[n], spec, root, out, None)
    else:
        known = set(spec) | (set(BUILD_DECLARED) if kind == "build" else set())
        for n in services:
            if n not in known:
                out.append("unknown_service %s.%s" % (kind, n))
        for n in spec:
            if n not in services:
                out.append("service_missing %s.%s" % (kind, n))
        if kind == "build":
            for n in BUILD_DECLARED:
                if n not in services:
                    out.append("service_missing %s.%s" % (kind, n))
        for n, svc in services.items():
            if n in spec:
                check_service(kind, n, svc, spec, root, out, None)
            elif kind == "build" and n in BUILD_DECLARED:
                check_service(kind, n, svc, spec, root, out, BUILD_DECLARED[n])
    # the project network (infra file) and the pod setting (every file)
    if kind in ("infra", "nfs"):
        net = (tree.get("networks") or {}).get("test-network") or {}
        expect(out, "%s.networks.test-network.name" % kind, net.get("name"), S("TI_PROJECT") + "_test-network")
        expect(out, "%s.networks.test-network.labels" % kind, as_dict(net.get("labels")), {"project": "catalogizer", "catalogizer.test_project": S("TI_PROJECT"), "catalogizer.test_root": S("TI_ROOT_HASH")})
    xp = tree.get("x-podman") or {}
    if str(xp.get("in_pod")).lower() != "false":
        out.append("in_pod_not_disabled %s (x-podman.in_pod must be false: podman-compose 1.5.0 defaults it to true and then creates an unlabelled pod)" % kind)
    return out


def apply_mutant(root, m):
    """m = dict(name, file, edits=[(old,new)...], expect='caught'|'survive', kind=<which scan>)"""
    texts = {}
    f = m["file"]
    t = open(os.path.join(root, f)).read()
    for old, new in m["edits"]:
        if t.count(old) != 1:
            return None, "anchor count %d for %r" % (t.count(old), old[:70])
        t = t.replace(old, new)
    texts[f] = t
    return texts, None


def run_mutants(modpath, root, only):
    import importlib.util
    spec = importlib.util.spec_from_file_location("compose_mutants", modpath)
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    bad = 0
    for m in mod.MUTANTS:
        if only and m["name"] not in only:
            continue
        texts, err = apply_mutant(root, m)
        if err:
            print("MUTANT %s BROKEN %s" % (m["name"], err)); bad += 1; continue
        kinds = m.get("scan") or [k for k, (_, files, _) in KINDS.items() if m["file"] in files and (k != "nfs" or m["file"].endswith("nfs.yml"))]
        findings = []
        for k in kinds:
            findings += scan(k, root, texts)
        want = m.get("expect", "caught")
        if want == "survive":
            if findings:
                print("MUTANT %s FAILED-BUT-IDENTITY %s" % (m["name"], findings[0])); bad += 1
            else:
                print("MUTANT %s SURVIVED-AS-REQUIRED" % m["name"])
        elif findings:
            print("MUTANT %s CAUGHT %s" % (m["name"], findings[0]))
        else:
            print("MUTANT %s SURVIVED" % m["name"]); bad += 1
    return 1 if bad else 0


def main(argv):
    root = os.getcwd()
    args = argv[1:]
    if "--root" in args:
        i = args.index("--root"); root = args[i + 1]; del args[i:i + 2]
    only = None
    if "--only" in args:
        i = args.index("--only"); only = set(args[i + 1].split(",")); del args[i:i + 2]
    if len(args) >= 2 and args[0] == "scan" and args[1] in KINDS:
        f = scan(args[1], root)
        print("\n".join(f))
        return 1 if f else 0
    if len(args) >= 2 and args[0] == "mutants":
        return run_mutants(args[1], root, only)
    print(__doc__, file=sys.stderr)
    return 2


if __name__ == "__main__":
    sys.exit(main(sys.argv))
