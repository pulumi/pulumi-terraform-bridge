#!/usr/bin/env python3
"""Exercise real golden tests with clean, contaminated, and concurrent plugin discovery.

Requires mise, git, make, and Python 3. Runs in disposable worktrees, including local
edits, so golden acceptance never modifies the caller's checkout or Pulumi home.
Logs are retained in the printed temporary directory on failure.
"""

import concurrent.futures
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile


ROOT = Path(__file__).resolve().parents[1]
TFGEN_TESTS = "^Test(ConvertViaPulumiCLI|PlainDocsParser)$"
DYNAMIC_TEST = "^TestSchemaGenerationFullDocs$"


def run(args, cwd, env=None, log=None):
    print(f"[{cwd.name}] {' '.join(map(str, args))}", flush=True)
    result = subprocess.run(
        list(map(str, args)), cwd=cwd, env=env, text=True,
        stdout=subprocess.PIPE, stderr=subprocess.STDOUT,
    )
    if log:
        log.write_text(result.stdout)
    if result.returncode:
        raise RuntimeError(f"Command failed, exit {result.returncode}:\n{result.stdout}")
    return result.stdout


def snapshot(root):
    # Include inline autogold values in Go sources and newly generated files too.
    paths = [root / "pkg/tfgen", root / "dynamic/testdata"]
    return {
        str(p.relative_to(root)): hashlib.sha256(p.read_bytes()).hexdigest()
        for directory in paths if directory.exists()
        for p in directory.rglob("*") if p.is_file()
    }


def poison(directory, binary, marker):
    directory.mkdir(parents=True, exist_ok=True)
    executable = directory / binary
    executable.write_text(f'#!/bin/sh\necho "isolation sentinel: {marker}" >&2\nexit 97\n')
    executable.chmod(0o755)


def plugin_pins(root):
    tools = json.loads(run(["mise", "ls", "--current", "--json"], root))
    pins = {}
    for tool, versions in tools.items():
        if not tool.startswith("vfox-pulumi:"):
            continue
        for version in versions:
            if Path(version["source"]["path"]).resolve() != root / "mise.toml":
                continue
            if not version["installed"]:
                raise RuntimeError(f"Run mise install first: {tool}")
            binary = tool.rsplit("/", 1)[1]
            if not binary.startswith("pulumi-converter-"):
                binary = binary.replace("pulumi-", "pulumi-resource-", 1)
            pins[binary] = Path(version["install_path"]) / "bin" / binary
            print(f"  {binary} {version['version']} {pins[binary]}", flush=True)
    if not pins:
        raise RuntimeError("No repo-owned mise plugin pins found")
    return pins


def verify_discovery(root, env, pins):
    inspected = subprocess.run(
        ["mise", "exec", "--", "python3", "-c",
         "import json, os; print(json.dumps(dict(os.environ)))"],
        cwd=root, env=env, text=True, stdout=subprocess.PIPE, check=True,
    )
    resolved = json.loads(inspected.stdout)
    for binary in pins:
        expected = root / ".pulumi/plugins/bin" / binary
        actual = shutil.which(binary, path=resolved["PATH"])
        if actual is None or Path(actual).resolve() != expected:
            raise RuntimeError(f"{binary}: selected {actual}, expected {expected}")
    home = Path(resolved["PULUMI_HOME"])
    if home.resolve() != root / ".pulumi" or not home.is_dir():
        raise RuntimeError("PULUMI_HOME was not isolated to the worktree")
    if resolved.get("PULUMI_DISABLE_AUTOMATIC_PLUGIN_ACQUISITION") != "true":
        raise RuntimeError("Automatic plugin acquisition is not disabled")
    listing = subprocess.run(
        ["mise", "exec", "--", "pulumi", "plugin", "ls", "--json"],
        cwd=root, env=env, text=True, stdout=subprocess.PIPE, check=True,
    )
    installed = json.loads(listing.stdout)
    actual = {f"pulumi-{p['kind']}-{p['name']}": p["version"] for p in installed}
    expected = {binary: path.parent.parent.name for binary, path in pins.items()}
    if actual != expected or len(installed) != len(pins):
        raise RuntimeError(f"Unexpected test cache: {actual}, expected {expected}")
    return resolved


def golden_tests(root, env, logs, label, accept=False):
    target = "test_accept" if accept else "test"
    update = " -args -update" if accept else ""
    run(["make", target, f"RUN_TEST_CMD=./pkg/tfgen -run '{TFGEN_TESTS}'{update}"],
        root, env, logs / f"{label}-tfgen.log")
    if accept:
        command = ["make", "test_accept", f"RUN_TEST_CMD=. -run '{DYNAMIC_TEST}'"]
    else:
        command = ["mise", "exec", "--", "go", "test", "-count=1", "-timeout=30m",
                   ".", "-run", DYNAMIC_TEST]
    run(command, root / "dynamic", env, logs / f"{label}-dynamic.log")


def check_missing_converter(root, resolved, logs):
    # Remove every ambient converter without modifying the mise tool installation.
    # Invoke the resolved CLI directly so mise cannot re-add it to PATH.
    cli = shutil.which("pulumi", path=resolved["PATH"])
    env = dict(resolved)
    env["PATH"] = os.pathsep.join(
        p for p in env["PATH"].split(os.pathsep)
        if not (Path(p) / "pulumi-converter-terraform").exists()
    )
    home = logs / "missing-plugin-home"
    home.mkdir()
    env["PULUMI_HOME"] = str(home)
    source = logs / "missing-plugin-source"
    source.mkdir()
    (source / "main.tf").write_text('resource "random_pet" "example" {}\n')
    result = subprocess.run(
        [cli, "convert", "--from", "terraform", "--language", "pcl", "--generate-only",
         "--out", str(logs / "missing-plugin-output")],
        cwd=source, env=env, text=True, stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT, timeout=60,
    )
    (logs / "missing-plugin.log").write_text(result.stdout)
    if result.returncode == 0 or "terraform" not in result.stdout.lower():
        raise RuntimeError(f"Missing converter did not fail clearly:\n{result.stdout}")
    if any(p.is_file() for p in (home / "plugins").rglob("*")):
        raise RuntimeError("Missing converter caused a plugin download")


def main():
    if os.name == "nt":
        raise RuntimeError("The sentinel executables require a POSIX host")
    scratch = Path(tempfile.mkdtemp(prefix="bridge-plugin-isolation-")).resolve()
    print(f"Isolation logs: {scratch}", flush=True)
    worktrees = []
    passed = False
    try:
        # Overlay the current checkout, including uncommitted implementation changes.
        files = run(["git", "ls-files", "--cached", "--others", "--exclude-standard", "-z"], ROOT).split("\0")
        for name in ["first", "second"]:
            root = scratch / name
            run(["git", "worktree", "add", "--detach", root, "HEAD"], ROOT)
            worktrees.append(root)
            for name in files:
                src = ROOT / name
                if name and src.is_file():
                    dst = root / name
                    dst.parent.mkdir(parents=True, exist_ok=True)
                    shutil.copy2(src, dst)
                elif name and not src.exists():
                    (root / name).unlink(missing_ok=True)
            run(["mise", "trust", root / "mise.toml"], root)

        pins = plugin_pins(worktrees[0])
        env = dict(os.environ)
        # Golden flags and diagnostics inherited from a caller must not affect the proof.
        for key in ["PULUMI_ACCEPT", "PULUMI_MISSING_DOCS_ERROR"]:
            env.pop(key, None)
        env["PULUMI_HOME"] = str(scratch / "user-home")
        before = [snapshot(root) for root in worktrees]
        for root in worktrees:
            run(["mise", "install"], root, env, scratch / f"{root.name}-setup.log")
        resolved = verify_discovery(worktrees[0], env, pins)
        check_missing_converter(worktrees[0], resolved, scratch)
        golden_tests(worktrees[0], env, scratch, "clean")

        # Add newer cached plugins and conflicting inherited PATH entries between runs.
        ambient = scratch / "ambient-bin"
        for binary in pins:
            poison(ambient, binary, "inherited PATH")
            kind_name = binary.removeprefix("pulumi-")
            for home in [scratch / "user-home", *(r / ".pulumi" for r in worktrees)]:
                poison(home / "plugins" / f"{kind_name}-v99.0.0", binary, "newer cached plugin")
        # Undeclared cache entries must not become extra mapping candidates either.
        for home in [scratch / "user-home", *(r / ".pulumi" for r in worktrees)]:
            for name in ["azure", "std"]:
                poison(home / "plugins" / f"resource-{name}-v99.0.0",
                       f"pulumi-resource-{name}", "undeclared cached plugin")
        env["PATH"] = str(ambient) + os.pathsep + env["PATH"]
        for root in worktrees:
            marker = root / ".pulumi/dynamic_tf_plugins/isolation-marker"
            marker.parent.mkdir(parents=True, exist_ok=True)
            marker.write_text("preserve non-Pulumi-plugin data")
            # Even with all tools installed, postinstall must remove stale cache entries.
            # Run setup before tests, never concurrently with readers of this cache.
            run(["mise", "install"], root, env, scratch / f"{root.name}-resync.log")
            verify_discovery(root, env, pins)
            if marker.read_text() != "preserve non-Pulumi-plugin data":
                raise RuntimeError("Plugin sync modified the separate Terraform provider cache")

        # Concurrent acceptance in separate worktrees exercises generation and dependency
        # sharing without introducing a race writing the same golden or coverage file.
        with concurrent.futures.ThreadPoolExecutor(max_workers=2) as executor:
            futures = [executor.submit(golden_tests, root, env, scratch, root.name, True)
                       for root in worktrees]
            for future in futures:
                future.result()
        golden_tests(worktrees[0], env, scratch, "repeat")
        for root, expected in zip(worktrees, before):
            actual = snapshot(root)
            if actual != expected:
                changed = sorted(k for k in actual.keys() | expected.keys()
                                 if actual.get(k) != expected.get(k))
                raise RuntimeError(f"Golden acceptance changed output in {root}: {changed}")
        passed = True
        print("PASS: clean, contaminated, concurrent acceptance, repeat, and missing-plugin checks", flush=True)
    finally:
        for root in worktrees:
            subprocess.run(["git", "worktree", "remove", "--force", str(root)], cwd=ROOT, check=True)
        if passed:
            shutil.rmtree(scratch)
        else:
            print(f"Failure logs retained at {scratch}", flush=True)


if __name__ == "__main__":
    main()
