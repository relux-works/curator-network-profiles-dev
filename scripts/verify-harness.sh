#!/bin/bash
# macOS only. One row, one non-forwarding sink, fresh HOME/configs.
# Python supervises descendants and records evidence before removing runtime state.
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
exec "${VERIFY_PYTHON:-/usr/bin/python3}" - "$ROOT" "$@" <<'PY'
import argparse
import datetime
import json
import os
from pathlib import Path
import re
import shutil
import signal
import subprocess
import sys
import tempfile
import time

root = Path(sys.argv[1])
parser = argparse.ArgumentParser(description="Run one harness row in a calibrated loopback-only SIGKILL sandbox.")
parser.add_argument("--lock-wrapper", required=True, type=Path)
parser.add_argument("--output-dir", required=True, type=Path, help="New directory for credential-free evidence")
parser.add_argument("--timeout", type=float, default=30)
parser.add_argument("--build-timeout", type=float, default=180, help="Maximum lock-wait/build seconds")
parser.add_argument("--proxy-state", choices=("reachable", "unreachable"), default="reachable")
parser.add_argument("--proxy-vars", choices=("generic", "lowercase", "uppercase", "all"), default="generic",
                    help="R3 coverage variant; every configured proxy still points at the sink")
parser.add_argument("--mcp-mode", choices=("inherit", "explicit"))
parser.add_argument("command", nargs=argparse.REMAINDER, help="After --: absolute executable and arguments")
args = parser.parse_args(sys.argv[2:])
command = args.command[1:] if args.command[:1] == ["--"] else args.command
if not command or not Path(command[0]).is_absolute() or args.timeout <= 0 or args.build_timeout <= 0:
    parser.error("Supply an absolute row executable after -- and a positive timeout")
if sys.platform != "darwin":
    parser.error("This helper requires macOS sandbox-exec")
command[0] = str(Path(command[0]).resolve())
python = str(Path(sys.executable).resolve())
if args.mcp_mode and args.proxy_vars != "generic":
    parser.error("MCP inheritance rows require the six-value generic patch")

operator_home = Path(os.environ["HOME"]).resolve()
protected = [operator_home / name for name in (".claude", ".codex", ".curator", ".config")]
protected += [operator_home / ".claude.json", operator_home / "Library/Keychains", Path("/Library/Keychains")]
def below(path, directory):
    return path == directory or directory in path.parents
for path in (Path(command[0]).resolve(), args.output_dir.resolve(), args.lock_wrapper.resolve()):
    if any(below(path, directory) for directory in protected):
        parser.error("Refusing executable, evidence, or lock wrapper in an operator config/keychain tree")
if args.output_dir.exists():
    parser.error("--output-dir must be a new directory")
args.output_dir.mkdir(parents=True, mode=0o700)
out = args.output_dir.resolve()
runtime = Path(tempfile.mkdtemp(prefix="harness-verification-", dir="/private/tmp"))
sink = None
build = None
row = None
known_pids = set()
summary = {"command": command, "proxy_state": args.proxy_state, "mcp_mode": args.mcp_mode,
           "proxy_vars": args.proxy_vars,
           "started_utc": datetime.datetime.now(datetime.timezone.utc).isoformat()}

def save_summary():
    (out / "summary.json").write_text(json.dumps(summary, indent=2) + "\n")

def processes():
    try:
        result = subprocess.run(["/bin/ps", "-axo", "pid=,ppid=,pgid="], capture_output=True, text=True, timeout=2)
    except subprocess.TimeoutExpired:
        summary.setdefault("sampling_errors", []).append("ps timed out")
        return []
    if result.returncode:
        summary.setdefault("sampling_errors", []).append("ps exit %s" % result.returncode)
        return []
    return [tuple(map(int, line.split())) for line in result.stdout.splitlines() if len(line.split()) == 3]

def row_pids():
    if row is None:
        return set()
    table = processes()
    current = {pid for pid, ppid, pgid in table if pgid == row.pid or pid == row.pid}
    # Retain descendants even if they detach after an earlier sample.
    current.update(pid for pid, ppid, pgid in table if pid in known_pids)
    while True:
        expanded = current | {pid for pid, ppid, pgid in table if ppid in current}
        if expanded == current:
            break
        current = expanded
    known_pids.update(current)
    return current

def terminate_row():
    if row is None:
        return
    pids = row_pids()
    try:
        os.killpg(row.pid, signal.SIGTERM)
    except ProcessLookupError:
        pass
    for pid in pids:
        try:
            os.kill(pid, signal.SIGTERM)
        except ProcessLookupError:
            pass
    deadline = time.monotonic() + 2
    while row_pids() and time.monotonic() < deadline:
        time.sleep(0.05)
    remaining = row_pids()
    if remaining:
        summary["supervisor_sigkill_pids"] = sorted(remaining)
        for pid in remaining:
            try:
                os.kill(pid, signal.SIGKILL)
            except ProcessLookupError:
                pass
    row.wait()

def cleanup():
    terminate_row()
    if build is not None and build.poll() is None:
        # Both the wrapper and Go are in this group; never release a lock while
        # leaving a build behind. A waiting wrapper has not acquired the lock.
        try:
            os.killpg(build.pid, signal.SIGTERM)
            build.wait(timeout=5)
        except (ProcessLookupError, subprocess.TimeoutExpired):
            try:
                os.killpg(build.pid, signal.SIGKILL)
            except ProcessLookupError:
                pass
            build.wait()
    if sink is not None:
        sink.terminate()
        try:
            sink.wait(timeout=2)
        except subprocess.TimeoutExpired:
            sink.kill()
            sink.wait()
    shutil.rmtree(runtime)

def interrupted(signum, frame):
    raise KeyboardInterrupt

signal.signal(signal.SIGTERM, interrupted)
signal.signal(signal.SIGINT, interrupted)
try:
    for name in ("home", "claude", "codex", "config", "cache", "data", "state", "tmp", "work", "go-cache", "go-mod"):
        (runtime / name).mkdir(mode=0o700)
    # No inherited auth variables, agent settings, SSH sockets, or runtime options.
    env = {"PATH": "/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin", "LANG": "en_US.UTF-8",
           "HOME": str(runtime / "home"), "CLAUDE_CONFIG_DIR": str(runtime / "claude"),
           "CODEX_HOME": str(runtime / "codex"), "XDG_CONFIG_HOME": str(runtime / "config"),
           "XDG_CACHE_HOME": str(runtime / "cache"), "XDG_DATA_HOME": str(runtime / "data"),
           "XDG_STATE_HOME": str(runtime / "state"), "TMPDIR": str(runtime / "tmp") + "/"}
    # Only the executable itself may be read from the operator HOME. In particular,
    # the installed ~/.codex binary is rejected above, not exempted here.
    profile = '''(version 1)
(allow default)
(deny network-outbound (with send-signal SIGKILL))
(allow network-outbound (remote ip "localhost:*"))
(allow network-outbound (remote unix-socket))
(deny network-outbound (literal "/private/var/run/mDNSResponder") (with send-signal SIGKILL))
(deny network-outbound (literal "/var/run/mDNSResponder") (with send-signal SIGKILL))
(deny mach-lookup (global-name "com.apple.securityd"))
'''
    profile += "(deny file-read* file-write* (subpath " + json.dumps(str(operator_home)) + "))\n"
    profile += "(allow file-read* (literal " + json.dumps(str(Path(command[0]).resolve())) + "))\n"
    # When VERIFY_PYTHON selects a user-installed interpreter, permit its code
    # installation (not the operator's configs) for the MCP stub's stdlib imports.
    if below(Path(sys.base_prefix).resolve(), operator_home):
        if any(below(Path(sys.base_prefix).resolve(), directory) for directory in protected):
            raise RuntimeError("Python code installation is in a protected operator directory")
        profile += "(allow file-read* (subpath " + json.dumps(str(Path(sys.base_prefix).resolve())) + "))\n"
    profile += '(deny file-read* file-write* (subpath "/Library/Keychains"))\n'
    sb = runtime / "loopback-only.sb"
    sb.write_text(profile)
    (out / "loopback-only.sb").write_text(profile)
    sandbox = ["/usr/bin/sandbox-exec", "-f", str(sb)]
    # Parse/activate the profile before the one deliberate, blocked external dial.
    activated = subprocess.run(sandbox + ["/usr/bin/true"], env=env, capture_output=True)
    if activated.returncode:
        raise RuntimeError("Sandbox activation failed: " + activated.stderr.decode(errors="replace"))
    calibration = subprocess.run(sandbox + ["/usr/bin/nc", "-n", "-z", "-w", "2", "1.1.1.1", "443"],
                                 env=env, capture_output=True, timeout=5)
    calibration_exit = 128 - calibration.returncode if calibration.returncode < 0 else calibration.returncode
    summary["calibration_exit"] = calibration_exit
    save_summary()
    (out / "calibration.stderr").write_bytes(calibration.stderr)
    if calibration.returncode != -signal.SIGKILL:
        raise RuntimeError("Unsafe detector: nc calibration must die with SIGKILL (137); no row started")
    build_env = dict(env, GOPROXY="off", GOSUMDB="off", GOENV="off", GOTOOLCHAIN="local",
                     GOFLAGS="-mod=readonly", GOCACHE=str(runtime / "go-cache"), GOMODCACHE=str(runtime / "go-mod"))
    # The supplied wrapper needs the operator HOME solely for its shared build lock.
    # Go itself receives the private environment and also runs inside the sandbox.
    lock_env = {"HOME": str(operator_home), "PATH": env["PATH"]}
    build_command = ["/bin/bash", str(args.lock_wrapper.resolve()), "/usr/bin/env", "-i"]
    build_command += [key + "=" + value for key, value in build_env.items()]
    build_command += sandbox + ["/usr/local/bin/go", "build", "-p", "1", "-o", str(runtime / "sink"),
                               "./internal/testproxy/cmd/loopback-proxy"]
    with (out / "build.log").open("wb") as log:
        build = subprocess.Popen(build_command, cwd=root, env=lock_env, stdout=log, stderr=log, start_new_session=True)
        try:
            build.wait(timeout=args.build_timeout)
        except subprocess.TimeoutExpired:
            raise RuntimeError("Shared build wrapper/Go exceeded --build-timeout; no harness row started")
    summary["build_exit"] = build.returncode
    if build.returncode:
        raise RuntimeError("Offline, locked sink build failed; see build.log")
    with (out / "sink.stdout").open("wb") as stdout, (out / "sink.log").open("wb") as stderr:
        # No --target: CONNECT always gets 502; no upstream is ever dialled.
        sink = subprocess.Popen(sandbox + [str(runtime / "sink"), "--id", "S", "--addr-file", str(runtime / "sink.addr")],
                                env=env, stdout=stdout, stderr=stderr)
    deadline = time.monotonic() + 5
    while not (runtime / "sink.addr").exists():
        if sink.poll() is not None or time.monotonic() > deadline:
            raise RuntimeError("Sink failed to become ready")
        time.sleep(0.05)
    endpoint = (runtime / "sink.addr").read_text().strip()
    if not re.fullmatch(r"http://127\.0\.0\.1:[0-9]+", endpoint):
        raise RuntimeError("Sink endpoint is not loopback")
    # For R2, stop this very sink: the patched URL stays a loopback sink URL.
    if args.proxy_state == "unreachable":
        sink.terminate()
        sink.wait(timeout=2)
    summary["endpoint"] = endpoint
    reserved = {"HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "FTP_PROXY", "NO_PROXY"}
    hostile = {"HTTPS_PROXY": "http://127.0.0.1:9", "NO_PROXY": "*", "Http_Proxy": "http://127.0.0.1:9",
               "all_proxy": "http://127.0.0.1:9", "FtP_PrOxY": "http://127.0.0.1:9"}
    env.update(hostile)
    env = {key: value for key, value in env.items() if key.upper() not in reserved}
    proxy_names = {"generic": ("HTTP_PROXY", "HTTPS_PROXY", "http_proxy", "https_proxy"),
                   "lowercase": ("http_proxy", "https_proxy"), "uppercase": ("HTTP_PROXY", "HTTPS_PROXY"),
                   "all": ("ALL_PROXY",)}
    patch = {key: endpoint for key in proxy_names[args.proxy_vars]}
    patch.update({key: "127.0.0.1,::1,localhost" for key in ("NO_PROXY", "no_proxy")})
    env.update(patch)
    (out / "patch.json").write_text(json.dumps({"hostile_before": hostile, "set_after": patch}, indent=2) + "\n")
    (out / "parent-env.json").write_text(json.dumps(env, indent=2, sort_keys=True) + "\n")
    stub = runtime / "mcp-stub.py"
    stub.write_text('''import json, os, sys
with open(''' + repr(str(runtime / "mcp-env.json")) + ''', "w") as f:
    json.dump(dict(os.environ), f, indent=2, sort_keys=True)
for line in sys.stdin:
    try:
        message = json.loads(line)
    except ValueError:
        continue
    if "id" not in message:
        continue
    method = message.get("method")
    if method == "initialize":
        result = {"protocolVersion": message.get("params", {}).get("protocolVersion", "2024-11-05"),
                  "capabilities": {"tools": {}}, "serverInfo": {"name": "env-stub", "version": "1"}}
    elif method == "tools/list":
        result = {"tools": []}
    elif method == "ping":
        result = {}
    else:
        print(json.dumps({"jsonrpc": "2.0", "id": message["id"], "error": {"code": -32601, "message": "Method not found"}}), flush=True)
        continue
    print(json.dumps({"jsonrpc": "2.0", "id": message["id"], "result": result}), flush=True)
''')
    server = {"command": python, "args": [str(stub)]}
    if args.mcp_mode == "explicit":
        server["env"] = patch
    mcp_json = runtime / "mcp.json"
    mcp_json.write_text(json.dumps({"mcpServers": {"stub": server}}, indent=2) + "\n")
    config = "[mcp_servers.stub]\ncommand = " + json.dumps(python) + "\nargs = [" + json.dumps(str(stub)) + "]\n"
    if args.mcp_mode == "explicit":
        config += "[mcp_servers.stub.env]\n" + "".join(key + " = " + json.dumps(value) + "\n" for key, value in patch.items())
    if args.mcp_mode:
        (runtime / "codex/config.toml").write_text(config)
    (out / "mcp.json").write_text(mcp_json.read_text())
    (out / "mcp-config.toml").write_text(config)
    command = [arg.replace("{mcp_json}", str(mcp_json)).replace("{stub}", str(stub)) for arg in command]
    summary["command"] = command
    login = re.compile(r"not logged in|please (?:run .*|log in|login)|(?:missing|no) (?:an? )?(?:api key|authentication)|run (?:/login|codex login)|(?:api key|authentication|login).*(?:is required|required to)|invalid api key.*(?:login|fix external api key)", re.I)
    non_loopback = []
    with (out / "row.stdout").open("wb") as stdout, (out / "row.stderr").open("wb") as stderr, (out / "lsof.log").open("w") as sockets:
        row = subprocess.Popen(sandbox + command, cwd=runtime / "work", env=env,
                               stdin=subprocess.DEVNULL, stdout=stdout, stderr=stderr, start_new_session=True)
        summary["row_pid"] = row.pid
        started = time.monotonic()
        while True:
            pids = row_pids()
            if pids:
                try:
                    sample = subprocess.run(["/usr/sbin/lsof", "-nP", "-a", "-p", ",".join(map(str, sorted(pids))), "-i"],
                                            capture_output=True, text=True, timeout=2)
                except subprocess.TimeoutExpired:
                    summary.setdefault("sampling_errors", []).append("lsof timed out")
                    sample = subprocess.CompletedProcess([], 1, "", "lsof timed out")
                if sample.stderr:
                    summary.setdefault("sampling_errors", []).append(sample.stderr.strip())
                sockets.write("t=%.3f pids=%s\n%s" % (time.monotonic() - started, sorted(pids), sample.stdout))
                sockets.flush()
                for line in sample.stdout.splitlines()[1:]:
                    # Include listeners/wildcards as unexpected; filter only explicit loopback endpoints.
                    addresses = re.findall(r"(?:\[[^]]+\]|[0-9.]+|\*):[0-9*]+", line)
                    if not addresses or any(not address.startswith(("127.", "[::1]:")) for address in addresses):
                        non_loopback.append(line)
            output = (out / "row.stdout").read_text(errors="replace") + (out / "row.stderr").read_text(errors="replace")
            if login.search(output):
                summary["needs_login"] = True
                summary["stop_reason"] = "needs login (Ivan's approval)"
                break
            if row.poll() is not None:
                break
            if time.monotonic() - started >= args.timeout:
                summary["stop_reason"] = "supervisor timeout"
                break
            time.sleep(0.5)
    if row.poll() is None:
        terminate_row()
    returncode = row.wait()
    summary["exit"] = 128 - returncode if returncode < 0 else returncode
    summary["root_sigkill"] = returncode == -signal.SIGKILL
    summary["non_loopback_socket_samples"] = sorted(set(non_loopback))
    summary["sink_requests"] = (out / "sink.log").read_text().splitlines()
    dump = runtime / "mcp-env.json"
    if dump.exists():
        child_env = json.loads(dump.read_text())
        shutil.copyfile(dump, out / "mcp-env.json")
        mismatch = {key: child_env.get(key) for key, value in patch.items() if child_env.get(key) != value}
        unexpected = {key: value for key, value in child_env.items() if key.upper() in reserved and key not in patch}
        summary["env_verdict"] = "PASS" if not mismatch and not unexpected else "FAIL"
        summary["env_mismatches"] = mismatch
        summary["unexpected_proxy_variables"] = unexpected
    else:
        summary["env_verdict"] = "no MCP env dump"
    summary["sampled_pids"] = sorted(known_pids)
    # The sandbox remains the protection for any descendant too short-lived to sample.
except (Exception, KeyboardInterrupt) as error:
    summary["helper_error"] = str(error) or "interrupted"
finally:
    try:
        cleanup()
    except Exception as error:
        summary["cleanup_error"] = str(error)
    save_summary()
    print(json.dumps(summary, indent=2))
sys.exit(1 if "helper_error" in summary or "cleanup_error" in summary else summary.get("exit", 1))
PY
