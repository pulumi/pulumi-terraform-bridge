#!/usr/bin/env python3
"""Build the bridge after setup.py installs the mise-managed tools and plugins."""

import subprocess
from pathlib import Path


REPO_ROOT = Path(__file__).resolve().parents[2]


if __name__ == "__main__":
    subprocess.run(["make", "build"], cwd=REPO_ROOT, check=True)
