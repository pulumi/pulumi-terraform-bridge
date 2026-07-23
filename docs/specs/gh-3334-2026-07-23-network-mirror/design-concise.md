# Network Mirror Support — Concise Design (for humans)

> **Same design as [`design.md`](./design.md), shortened for people.**  
> Agents and implementers should always read the full [`design.md`](./design.md). This file is a readable overview of the problem, decisions, and examples — not a substitute for the full spec.

**Status:** Draft · **Date:** 2026-07-23 · **Scope:** Dynamic `terraform-provider` only  
**Issues:** [#3334](https://github.com/pulumi/pulumi-terraform-bridge/issues/3334), [pulumi-terraform-provider#106](https://github.com/pulumi/pulumi-terraform-provider/issues/106)

---

## Problem

Air-gapped / enterprise networks often cannot reach `registry.terraform.io` or `registry.opentofu.org`. Terraform users already fix this with a [network mirror](https://developer.hashicorp.com/terraform/internals/provider-network-mirror-protocol) in `.terraformrc`.

Pulumi’s dynamic `terraform-provider` still does registry `.well-known` discovery, so installs fail even when a working mirror exists:

```text
Get "https://registry.terraform.io/.well-known/terraform.json": Forbidden
```

`PULUMI_PLUGIN_DOWNLOAD_URL_OVERRIDES` does **not** help — it rewrites Pulumi plugin downloads, not Terraform provider binaries fetched by the dynamic bridge.

---

## What we want

1. Download TF providers via the **network mirror protocol** and **skip** `.well-known` when a mirror is configured.
2. Machine-wide escape hatch for CI / air-gap hosts (env var).
3. Per-package, durable config that survives `package add` → `install` → `preview`/`up` **without** needing the env var on every machine.
4. Auth tokens never in `Pulumi.yaml` or state.
5. Behavioral parity with Terraform `provider_installation` outcomes — via **Pulumi-native** knobs only (no `.terraformrc` parser).

**Out of scope:** static bridged providers, `dev_overrides`, full `.terraformrc` parity, changing Pulumi plugin download.

---

## Phases

| Phase | Ships | Closes |
|-------|--------|--------|
| **1** | `MirrorSource` + `PULUMI_TF_NETWORK_MIRROR_OVERRIDES` (hosts / `*` / TF globs / `!deny`) + `TF_TOKEN_*` | Machine-side mirror + auth |
| **2** | `--provider-mirror` + persist in parameterized `Value` | **#3334** (durable per-package) |
| **3** (later) | Optional: Pulumi credentials store, archive hash verify, `filesystem_mirror` | Polish |

Phase 1 = one PR. Phase 2 = follow-up PR.

---

## Key decisions

| | Choice | Why |
|---|--------|-----|
| Config | Pulumi-native only; **no** `.terraformrc` parser | Avoid dual sources of truth |
| Env | One var: `PULUMI_TF_NETWORK_MIRROR_OVERRIDES` (no separate catch-all URL env) | `*=url` is enough |
| Patterns | Exact host, `*`, TF path globs, `!pattern` deny — **no** Go/RE2 regex | One language, TF-shaped |
| Durability | Persist mirror URL in SDK-embedded `Value` (Phase 2) | Runtime uses `Value`, not raw YAML; env-only fails on cold cache |
| Auth | `TF_TOKEN_<host>` (Phase 1); never a flag / yaml field | Secrets must not be committed |
| Precedence | `--provider-mirror` > OVERRIDES > registry discovery | Flag = project intent; env = machine policy |
| Default host | Bare names still resolve to `registry.opentofu.org` | No guessing from mirror URL — qualify `registry.terraform.io/...` when the mirror is TF-only |
| Same-host Artifactory | **No** auto-skip for private providers on the mirror hostname | Users scope overrides or use `!private-host/*` |
| Trust | Trust the mirror in v1; hash verify later | Same trust model as Terraform |

---

## How users configure it

### Phase 1 — machine / CI

```bash
export PULUMI_TF_NETWORK_MIRROR_OVERRIDES=\
'*=https://artifactory.example.com/api/terraform/providers/'

# Better when private providers share the Artifactory host:
export PULUMI_TF_NETWORK_MIRROR_OVERRIDES=\
'registry.terraform.io=https://…/providers/,registry.opentofu.org=https://…/providers/'
# or: '*=https://…/providers/,!myartifactory.example.com/*'

export TF_TOKEN_artifactory_example_com='********'   # if mirror needs auth

pulumi package add terraform-provider registry.terraform.io/hashicorp/random 3.6.0
```

On match: skip `.well-known`, hit mirror paths like:

```text
GET {M}/registry.terraform.io/hashicorp/random/index.json
GET {M}/registry.terraform.io/hashicorp/random/3.6.0.json
```

### Phase 2 — durable per package (preferred for projects)

```bash
pulumi package add terraform-provider -- \
  registry.terraform.io/hashicorp/random 3.6.0 \
  --provider-mirror https://artifactory.example.com/api/terraform/providers/
```

Lands in `Pulumi.yaml` `parameters`, and the mirror URL is also embedded in the generated SDK `Value`. Success check: clean machine, empty plugin cache, **no** env → `pulumi up` still uses the mirror.

### Precedence

| Flag | Env | Source used |
|------|-----|-------------|
| set | anything | Flag |
| unset | set | Env |
| unset | unset | Registry discovery |

---

## Gotchas worth knowing

**1. Bare name ≠ Terraform registry**

`hashicorp/random` defaults to `registry.opentofu.org/...`. If the mirror only has Terraform-registry layout, qualify the host:

```bash
# Wrong path on a TF-only mirror:
pulumi package add … hashicorp/random … --provider-mirror "$MIRROR"

# Right:
pulumi package add … registry.terraform.io/hashicorp/random … --provider-mirror "$MIRROR"
```

**2. Same Artifactory host: public mirror + private providers**

Common setup: one DNS name serves (a) network-mirror layout for public registries and (b) registry protocol for `myartifactory…/myorg/custom`.

- Catch-all `*=` forces private addresses through the mirror protocol → often 404.
- Prefer scoped host overrides, `!myartifactory…/*`, or put `--provider-mirror` only on public packages.

**3. Cache can hide bad config**

Cache keys by address + version, not download source. Clear `PULUMI_DYNAMIC_TF_PLUGIN_CACHE_DIR` when validating mirror setups.

**4. Tokens never in yaml**

No `--provider-mirror-token`. Use `TF_TOKEN_<host>` (`.` → `_` in the host).

---

## Terraform → Pulumi (behavioral mapping)

| Terraform intent | Pulumi |
|------------------|--------|
| Catch-all `network_mirror { url }` | `OVERRIDES='*=url'` or `--provider-mirror` |
| Mirror only `registry.terraform.io` | `OVERRIDES='registry.terraform.io=url'` |
| Mirror public, keep private host direct | Scoped hosts or `*=url,!private-host/*` |
| Namespace allowlist | `registry.terraform.io/hashicorp/*=url` |
| Exclude from mirror | `!pattern` (checked before positives) |
| Multiple mirrors | Multiple `pattern=url` entries |
| Auth | `TF_TOKEN_*` (Phase 1); credentials.json later |
| `filesystem_mirror` / read `.terraformrc` | Not in MVP (Phase 3 / maybe never) |

---

## What we checked / concluded

- Env-only forever is **not** enough for #3334 — runtime needs the mirror in `Value`.
- Encoding the mirror into the provider address string is the wrong layer.
- Parsing `.terraformrc` first delays the durable fix and creates dual config.
- Typical enterprise `network_mirror` + `direct` setups are covered by OVERRIDES + `--provider-mirror`.
- Full Terraform `provider_installation` parity is **not** the goal (no `filesystem_mirror` / `dev_overrides` / multi-source version racing in MVP).

---

## Review focus (for maintainers)

1. Phase 1 as one PR: OVERRIDES (globs + `!`) + `TF_TOKEN_*`
2. Phase 2 follow-up: `--provider-mirror` + `Value`
3. No `.terraformrc` / no separate `MIRROR_URL` / no same-host auto-skip

See also [`maintainer-questions.md`](./maintainer-questions.md) and the full [`design.md`](./design.md).
