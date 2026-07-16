#!/usr/bin/env python3
"""Prepare a fresh OpenInspect sandbox for this repository.

Boot sequence:
  1. Install mise, trust the repo config, and install repo tools.
  2. Run repository setup hooks from .openinspect/setup.d in filename order.
  3. Run .openinspect/setup.local.py when present.
"""

import logging
import os
import shutil
import subprocess
import sys
import time
from contextlib import contextmanager
from pathlib import Path


log = logging.getLogger("setup")
logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(phase)s] %(message)s",
    datefmt="%H:%M:%S",
    stream=sys.stdout,
)

_log_factory = logging.getLogRecordFactory()
_current_phase = "setup"


def _record_factory(*args, **kwargs):
    record = _log_factory(*args, **kwargs)
    record.phase = _current_phase
    return record


logging.setLogRecordFactory(_record_factory)


@contextmanager
def phase(name: str, description: str = ""):
    global _current_phase
    previous_phase = _current_phase
    _current_phase = name
    label = description or name
    log.info("Starting: %s", label)
    start = time.monotonic()
    try:
        yield
    except Exception:
        elapsed = time.monotonic() - start
        log.error("FAILED after %.1fs: %s", elapsed, label)
        raise
    else:
        elapsed = time.monotonic() - start
        log.info("Done in %.1fs: %s", elapsed, label)
    finally:
        _current_phase = previous_phase


REPO_ROOT = Path(__file__).resolve().parent.parent
OPENINSPECT_DIR = REPO_ROOT / ".openinspect"


def run(command, **kwargs):
    command_string = command if isinstance(command, str) else " ".join(command)
    log.info("$ %s", command_string)
    kwargs.setdefault("check", True)
    kwargs.setdefault("cwd", REPO_ROOT)
    return subprocess.run(command, shell=isinstance(command, str), **kwargs)


def setup_mise():
    os.environ.setdefault("MISE_FETCH_REMOTE_VERSIONS_TIMEOUT", "10m")
    os.environ.setdefault("MISE_HTTP_TIMEOUT", "10m")

    if not shutil.which("mise"):
        run("curl -fsSL https://mise.run | MISE_INSTALL_PATH=/usr/local/bin/mise sh")
    else:
        log.info("mise already installed, skipping download")

    run("mise trust --yes")
    run("mise install --yes")

    mise_data = os.environ.get(
        "MISE_DATA_DIR", os.path.expanduser("~/.local/share/mise")
    )
    shims = f"{mise_data}/shims"
    if shims not in os.environ.get("PATH", ""):
        os.environ["PATH"] = f"{shims}:{os.environ.get('PATH', '')}"

    export_line = f'export PATH="{shims}:$PATH"'
    for shell_config in [Path.home() / ".bashrc", Path.home() / ".profile"]:
        if shell_config.exists() and export_line in shell_config.read_text():
            continue
        with shell_config.open("a") as config_file:
            config_file.write(
                f"\n# Added by .openinspect/setup.py - mise shims\n{export_line}\n"
            )


def run_hook(path: Path):
    run([sys.executable, str(path)])


def run_setup_hooks():
    hooks_dir = OPENINSPECT_DIR / "setup.d"
    if hooks_dir.exists():
        for hook in sorted(hooks_dir.glob("*.py")):
            with phase("hook", f"Run {hook.relative_to(REPO_ROOT)}"):
                run_hook(hook)

    local_hook = OPENINSPECT_DIR / "setup.local.py"
    if local_hook.exists():
        with phase("hook", f"Run {local_hook.relative_to(REPO_ROOT)}"):
            run_hook(local_hook)


if __name__ == "__main__":
    total_start = time.monotonic()
    log.info("=== Sandbox setup starting ===")

    with phase("mise", "Install mise and trust repo config"):
        setup_mise()

    run_setup_hooks()

    total_elapsed = time.monotonic() - total_start
    minutes = int(total_elapsed // 60)
    seconds = int(total_elapsed % 60)
    log.info("=== Sandbox setup complete in %dm%ds ===", minutes, seconds)
