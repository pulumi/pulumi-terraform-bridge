#!/usr/bin/env python3
"""Build the bridge and install the Pulumi plugins required by its tests."""

import subprocess
from pathlib import Path


REPO_ROOT = Path(__file__).resolve().parents[2]


if __name__ == "__main__":
    subprocess.run(["make", "build"], cwd=REPO_ROOT, check=True)
    subprocess.run(["make", "install_plugins"], cwd=REPO_ROOT, check=True)
