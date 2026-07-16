# OpenInspect setup

This directory prepares the Pulumi Terraform Bridge for review in a fresh OpenInspect sandbox.

- `settings.json` makes tools installed by `mise` available on `PATH`.
- `setup.py` installs and activates the tools declared in [`mise.toml`](../mise.toml), then runs setup hooks.
- `setup.d/10-build-and-install-plugins.py` runs `make build` and `make install_plugins` so the bridge is buildable and its test plugins are available.

Repository-specific setup hooks belong in `setup.d` and run in filename order. An optional final hook may be placed at `setup.local.py`.

Tests are intentionally not part of sandbox boot. Run the targeted validation described in [`AGENTS.md`](../AGENTS.md) for the code under review.
