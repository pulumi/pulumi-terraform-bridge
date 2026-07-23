# Network Mirror Support for Dynamic Terraform Providers

**Status:** Draft for review  
**Date:** 2026-07-23  
**Authors:** Discussion on PR [#3463](https://github.com/pulumi/pulumi-terraform-bridge/pull/3463), issues [#3334](https://github.com/pulumi/pulumi-terraform-bridge/issues/3334) and [pulumi-terraform-provider#106](https://github.com/pulumi/pulumi-terraform-provider/issues/106)  
**Scope:** Dynamic `terraform-provider` path in `pulumi-terraform-bridge`  
**Audience:** Maintainers and contributors reviewing design before / during implementation

---

## 1. Executive summary

Air-gapped and enterprise environments often cannot reach public Terraform or OpenTofu registries. Those environments already use the [Provider Network Mirror Protocol](https://developer.hashicorp.com/terraform/internals/provider-network-mirror-protocol) (also documented by [OpenTofu](https://opentofu.org/docs/internals/provider-network-mirror-protocol/)) via `.terraformrc` / `.tofurc` `provider_installation` blocks.

Pulumi’s dynamic `terraform-provider` currently resolves and downloads upstream Terraform providers through registry service discovery (`.well-known/terraform.json`). That call fails when the public registry is unreachable, even when a working network mirror exists.

This document defines a **target architecture** and a **phased plan** to support network mirrors:

| Phase | Capability | Primary outcome |
|-------|------------|-----------------|
| **0** | `PULUMI_TF_NETWORK_MIRROR_URL` | Unblock air-gapped installs via env (PR #3463) |
| **1** | `--provider-mirror` + persist in parameterized `Value` | Durable per-package config; closes #3334 |
| **2** | `TF_TOKEN_*` auth for mirror HTTP | Enterprise Artifactory / authenticated mirrors |
| **3** | Hash verification; include/exclude routing | Closer to TF `provider_installation` semantics |
| **4** | Optional later: `filesystem_mirror`, `.terraformrc` parsing | Only if demand justifies the surface |

**Recommended approach:** ship Phase 0, then Phase 1 as a follow-up PR. Phases 2–4 are documented targets, not blockers for the first durable close of #3334.

For a concrete catalog of scenarios that work vs fail (including Artifactory mirror + private packages on the same host), see **§8A**.

---

## 2. Problem statement

### 2.1 What fails today

When a user runs:

```bash
pulumi package add terraform-provider hashicorp/random
# or later:
pulumi install
pulumi up
```

the dynamic bridge may attempt registry discovery against `registry.terraform.io` or `registry.opentofu.org`. In restricted networks this fails, for example:

```text
could not connect to registry.terraform.io: failed to request discovery document:
Get "https://registry.terraform.io/.well-known/terraform.json": Forbidden
```

Terraform / OpenTofu users solve this with CLI config such as:

```hcl
provider_installation {
  network_mirror {
    url = "https://artifactory.example.com/api/terraform/providers/"
  }
}
```

Pulumi has no equivalent for **Terraform provider binaries** downloaded by the dynamic bridge.

### 2.2 Why existing Pulumi env vars are not enough

`PULUMI_PLUGIN_DOWNLOAD_URL_OVERRIDES` rewrites download locations for **Pulumi plugins**. It does not change how the dynamic bridge resolves **Terraform** providers after `terraform-provider` itself is installed.

Member guidance on #3334:

> The solution will require passing in data equivalent to the `provider_installation` block as a flag to the parameterization. `PULUMI_PLUGIN_DOWNLOAD_URL_OVERRIDES` won't do what we need.

### 2.3 Related work

| Item | Role |
|------|------|
| [pulumi-terraform-provider#106](https://github.com/pulumi/pulumi-terraform-provider/issues/106) | Air-gap / mirror motivation (user report) |
| [bridge#3334](https://github.com/pulumi/pulumi-terraform-bridge/issues/3334) | Feature request for `network_mirror` / `provider_installation` |
| [PR #3463](https://github.com/pulumi/pulumi-terraform-bridge/pull/3463) | Phase 0: env-var mirror support |
| iwahbe on #3463 | Env var and `--provider-mirror` are **complementary** |

---

## 3. Goals and non-goals

### 3.1 Goals

1. Download Terraform providers via the network mirror protocol **without** calling registry `.well-known` discovery when a mirror is configured.
2. Support providers addressed on **either** `registry.terraform.io` **or** `registry.opentofu.org` (and other registry hostnames encoded in the provider address).
3. Provide a **machine-wide** escape hatch (environment variable) for CI / air-gapped hosts.
4. Provide a **per-package, reproducible** configuration that survives `pulumi package add`, `pulumi install`, and runtime `preview` / `up` without requiring the env var.
5. Keep secrets out of `Pulumi.yaml` and out of Pulumi state.
6. Align configuration UX with existing dynamic-provider parameterization (`pulumi package add terraform-provider -- …`).
7. Document a clear path toward richer `provider_installation`-like features without blocking the MVP.

### 3.2 Non-goals

- Changing how **statically bridged** providers are published or installed (they ship as Pulumi plugins; no TF registry download at user time).
- Changing `PULUMI_PLUGIN_DOWNLOAD_URL_OVERRIDES` or Pulumi plugin download generally.
- Requiring Pulumi CLI changes for `--provider-mirror` (CLI already stores opaque provider parameters).
- Full parity with `.terraformrc` / `.tofurc` `provider_installation` in the MVP.
- Implementing `dev_overrides`.
- Replacing or re-implementing Terraform’s provider lockfile / trust model beyond what we already do on the registry path.

---

## 4. Design decisions (locked)

These decisions were made during design review. They constrain implementation.

| # | Decision | Choice | Rationale |
|---|----------|--------|-----------|
| D1 | Runtime durability | Persist mirror URL in parameterized **`Value`** (SDK-embedded), not only in `Pulumi.yaml` | Runtime uses `ParameterizeValue`; env-only fails on cache miss in air-gap |
| D2 | Auth mechanism | Client-local **`TF_TOKEN_<host>`** (TF-compatible). Never a parameterization flag | Tokens must not appear in checked-in `Pulumi.yaml` |
| D3 | Registry host default | Keep today’s default (`registry.opentofu.org` for bare names). Users qualify `registry.terraform.io/...` when needed | Explicit is clearer than fragile inference from mirror URL |
| D4 | Routing | v1 applies the configured mirror to **all** remote provider downloads | Simplest correct air-gap behavior |
| D5 | Flag shape | Single `--provider-mirror <URL>` (`string`) | Matches iwahbe’s example; multi-mirror later if needed |
| D6 | Hash verification | Trust mirror for v1; verify later | Mirror is already a trusted distribution channel |
| D7 | Config end-state | Flag + env are the permanent public API unless demand proves `.terraformrc` needed | Avoid large CLI-config parser surface |
| D8 | Precedence | **flag > env > default registry** | Per-package intent wins over machine default |
| D9 | Delivery | Phase 0 = current PR; Phase 1 = follow-up PR based on Phase 0 | Keep reviews focused; complementary features |

---

## 5. Background: how resolution works today

### 5.1 Parameterization entry points

Dynamic provider parameterization lives in `dynamic/main.go` (`XParamaterize`) and has two input shapes:

| Request type | When used | Source of config |
|--------------|-----------|------------------|
| `ParameterizeArgs` | `pulumi package add`, `pulumi install` replaying `packages.*.parameters` | Raw CLI / YAML string args |
| `ParameterizeValue` | Program runtime (`preview` / `up`) via generated SDK | JSON bytes embedded by `SchemaPostProcessor` |

Relevant code:

- Args parsing: `dynamic/parameterize/args.go`
- Value (SDK embed): `dynamic/parameterize/value.go`
- Embed into schema: `dynamic/info.go` (`Parameter: value.Marshal()`)
- Provider download: `dynamic/internal/shim/run/loader.go` (`NamedProvider`, `getProviderServer`, `getProviderFromMirror`)

### 5.2 Critical durability gap (Phase 0 alone)

Phase 0 wires the mirror only through:

```text
os.Getenv("PULUMI_TF_NETWORK_MIRROR_URL") → getProviderFromMirror(...)
```

That works when the env var is set. It is **not** enough for a durable project config:

1. `package add` with only env set does not record the mirror in `parameters` or `Value`.
2. Later, on another machine (or CI job) with empty plugin cache and no env var, runtime `ParameterizeValue` reconstructs args **without** a mirror and falls back to registry discovery.
3. Air-gap failure returns.

**Therefore Phase 1 must round-trip the mirror URL through both `Args` and `Value`.**

### 5.3 CLI storage (no CLI change required)

`pulumi package add` already stores everything after the provider name as opaque parameters in `Pulumi.yaml` (`PackageSpec.Parameters`). Dash-style flags are supported via `--`:

```bash
pulumi package add terraform-provider -- \
  registry.terraform.io/hashicorp/random 3.6.0 \
  --provider-mirror https://mirror.example/providers/
```

→

```yaml
packages:
  random:
    source: terraform-provider
    version: <bridge-version>
    parameters:
      - registry.terraform.io/hashicorp/random
      - 3.6.0
      - --provider-mirror
      - https://mirror.example/providers/
```

The bridge’s cobra parser in `dynamic/parameterize/args.go` must learn `--provider-mirror`. The Pulumi CLI does not need a first-class flag.

---

## 6. Network mirror protocol and registry hosts

### 6.1 Protocol endpoints

Given mirror base URL `M` and provider address `hostname/namespace/type`:

| Purpose | Request |
|---------|---------|
| List versions | `GET {M}/{hostname}/{namespace}/{type}/index.json` |
| Package metadata | `GET {M}/{hostname}/{namespace}/{type}/{version}.json` |
| Archive | URL(s) returned in version JSON (absolute or relative) |

Implementation: `pkg/vendored/opentofu/getproviders/mirror_source.go` (`MirrorSource.providerURL`).

### 6.2 Terraform vs OpenTofu addresses

The provider **address hostname is part of the path**. One physical mirror can serve both ecosystems:

```text
{M}/registry.terraform.io/hashicorp/random/index.json
{M}/registry.opentofu.org/hashicorp/random/index.json
```

This matches Artifacts/Artifactory-style Terraform remote repository layouts that preserve registry host prefixes.

### 6.3 Bare name default (D3)

Unresolved short names (e.g. `hashicorp/random`) currently default to **`registry.opentofu.org`** via `regaddr` parsing used by the loader.

Implications:

| User input | Resolved address | Mirror object path |
|------------|------------------|--------------------|
| `hashicorp/random` | `registry.opentofu.org/hashicorp/random` | `…/registry.opentofu.org/hashicorp/random/…` |
| `registry.opentofu.org/hashicorp/random` | same | same |
| `registry.terraform.io/hashicorp/random` | TF registry address | `…/registry.terraform.io/hashicorp/random/…` |

**Operational guidance:** if the corporate mirror only mirrors the Terraform registry layout, users must pass a fully qualified `registry.terraform.io/...` source. Docs and error messages should state this explicitly (Phase 1 documentation task).

We will **not** infer the registry host from the mirror URL.

---

## 7. Target architecture

### 7.1 Configuration surfaces (ranked)

1. **Per-package `--provider-mirror <URL>`** (primary, durable)
   - Stored in `Pulumi.yaml` `parameters`
   - Persisted in parameterized `Value` for runtime
2. **Environment variable `PULUMI_TF_NETWORK_MIRROR_URL`** (machine / CI default)
3. **Default registry + service discovery** (unchanged)

Precedence (D8):

```text
explicit --provider-mirror  >  PULUMI_TF_NETWORK_MIRROR_URL  >  registry discovery
```

### 7.2 Credentials surface (separate from mirror URL)

| Kind | Where it lives | Checked in? |
|------|----------------|-------------|
| Mirror URL | `parameters` / `Value` / optional env | URL yes; OK |
| Auth token | `TF_TOKEN_<host>` (process env), future: netrc | **Never** in yaml/state |

Hostname encoding for `TF_TOKEN_*` follows Terraform/OpenTofu rules (`.` → `_`, etc.). Example:

```bash
export TF_TOKEN_artifactory_example_com=********
export PULUMI_TF_NETWORK_MIRROR_URL=https://artifactory.example.com/api/terraform/providers/
```

Phase 2 defines exactly which host the token is looked up against (mirror host vs provider registry host). See §11.

### 7.3 Data flow

```mermaid
flowchart TD
  subgraph configure [Configure]
    CLI["pulumi package add terraform-provider -- NAME VER --provider-mirror URL"]
    YAML["Pulumi.yaml packages.*.parameters"]
    ENV["PULUMI_TF_NETWORK_MIRROR_URL"]
    TOKEN["TF_TOKEN_* / local creds"]
  end

  subgraph parameterize [Parameterize]
    ARGS["ParameterizeArgs → ParseArgs"]
    VAL["ParameterizeValue → ParseValue → IntoArgs"]
    MERGE["Args including Mirror field"]
  end

  subgraph resolve [Resolve download source]
    PREC{"Mirror from Args?\nelse env?\nelse default"}
    MS["MirrorSource\n{base}/{host}/{ns}/{type}/…"]
    REG["RegistrySource + disco\n.well-known"]
  end

  subgraph cache [Cache and run]
    CACHE["providercache under PULUMI_DYNAMIC_TF_PLUGIN_CACHE_DIR"]
    RUN["runProvider"]
  end

  subgraph embed [Persist for runtime]
    VALUE["parameterize.Value includes Mirror"]
    SDK["Generated SDK embeds Value bytes"]
  end

  CLI --> YAML
  YAML --> ARGS
  SDK --> VAL
  ARGS --> MERGE
  VAL --> MERGE
  ENV --> PREC
  MERGE --> PREC
  PREC -->|mirror| MS
  PREC -->|default| REG
  TOKEN -.->|Phase 2| MS
  MS --> CACHE
  REG --> CACHE
  CACHE --> RUN
  MERGE --> VALUE
  VALUE --> SDK
```

### 7.4 Proposed data model changes (Phase 1)

**`parameterize.Args`** — add mirror URL (top-level or on `RemoteArgs`; top-level is slightly simpler for `IntoArgs` round-trip):

```go
type Args struct {
    Remote       *RemoteArgs
    Local        *LocalArgs
    Includes     []string
    Excludes     []string
    ProviderName string
    Mirror       string // empty = unset; --provider-mirror
}
```

**CLI flag:**

```go
cmd.Flags().StringVar(&mirror, "provider-mirror", "",
    "Terraform network mirror base URL (skips registry service discovery)")
```

**`parameterize.Value`** — persist mirror for runtime:

```go
type Value struct {
    Remote       *RemoteValue `json:"remote,omitempty"`
    Local        *LocalValue  `json:"local,omitempty"`
    Includes     []string     `json:"includes,omitempty"`
    Excludes     []string     `json:"excludes,omitempty"`
    ProviderName string       `json:"providerName,omitempty"`
    Mirror       string       `json:"mirror,omitempty"`
}
```

**`IntoArgs` / `XParamaterize`:** copy `Mirror` both directions. Local providers ignore `Mirror` (no download).

**`run.NamedProvider`:** accept optional mirror argument (or resolve inside `getProvider`):

```text
mirror := args.Mirror
if mirror == "" {
    mirror = os.Getenv("PULUMI_TF_NETWORK_MIRROR_URL")
}
if mirror != "" {
    return getProviderFromMirror(...)
}
return getProviderServer(... disco ...)
```

### 7.5 Local providers

`LocalArgs` / path-based providers do not use the mirror. If `--provider-mirror` is passed with a local path, prefer: **error** (invalid combination) rather than silently ignoring, to avoid false confidence.

---

## 8. User experience

### 8.1 Phase 0 — environment variable

```bash
export PULUMI_TF_NETWORK_MIRROR_URL=https://artifactory.example.com/api/terraform/providers/
pulumi package add terraform-provider registry.terraform.io/hashicorp/random 3.6.0
pulumi up
```

Behavior when set:

1. Skip `.well-known` discovery.
2. Query mirror with network mirror protocol.
3. Install into the existing dynamic TF plugin cache.
4. Run the provider as today.

### 8.2 Phase 1 — per-package flag

```bash
pulumi package add terraform-provider -- \
  registry.terraform.io/hashicorp/random 3.6.0 \
  --provider-mirror https://artifactory.example.com/api/terraform/providers/
```

Resulting `Pulumi.yaml` fragment:

```yaml
packages:
  random:
    source: terraform-provider
    parameters:
      - registry.terraform.io/hashicorp/random
      - 3.6.0
      - --provider-mirror
      - https://artifactory.example.com/api/terraform/providers/
```

**Success criterion:** on a clean machine with empty `PULUMI_DYNAMIC_TF_PLUGIN_CACHE_DIR`, **without** `PULUMI_TF_NETWORK_MIRROR_URL`, `pulumi preview` / `up` still downloads from the mirror because the SDK-embedded `Value` carries `mirror`.

### 8.3 Mixing env and flag

| Scenario | Effective mirror |
|----------|------------------|
| Flag set, env set | Flag |
| Flag set, env unset | Flag |
| Flag unset, env set | Env |
| Both unset | Registry discovery |

### 8.4 OpenTofu-addressed providers

```bash
pulumi package add terraform-provider -- \
  hashicorp/random 3.6.0 \
  --provider-mirror https://mirror.example/providers/
# resolves to registry.opentofu.org/hashicorp/random
```

Mirror must contain the OpenTofu host prefix path, or the user must use an explicit Terraform host if that is what the mirror publishes.

---

## 8A. Examples: what works and what does not

Assumptions used below (unless a row says otherwise):

| Symbol | Meaning |
|--------|---------|
| `MIRROR` | `https://myartifactory.example.com/artifactory/api/terraform/providers/` |
| Mirror contents | Publishes **network-mirror** layout for `registry.terraform.io/…` (and optionally `registry.opentofu.org/…`) |
| Private provider | `myartifactory.example.com/myorg/custom` available via **registry protocol** on that host, **not** under the mirror repo layout |
| Phases | ✅ = supported in that phase · ❌ = fails / unsupported · ⚠️ = works only with caveats · 🔜 = planned later |

Legend for result columns: **P0** = env only (PR #3463), **P1** = `--provider-mirror` + `Value` persistence, **P2** = `TF_TOKEN_*`, **P3** = include/exclude.

### 8A.1 Public providers from a network mirror

| # | What you do | P0 | P1 | Notes |
|---|-------------|----|----|-------|
| 1 | `export PULUMI_TF_NETWORK_MIRROR_URL=MIRROR` then add `registry.terraform.io/hashicorp/random` | ✅ | ✅ | Classic air-gap path; skips `.well-known` |
| 2 | Add with `--provider-mirror MIRROR` and fully qualified TF address | ❌ | ✅ | Preferred project-local config; no env required after Parameterize |
| 3 | Add with `--provider-mirror MIRROR`, then later `pulumi up` on clean cache **without** env | ❌ | ✅ | Requires `Value.mirror` round-trip (D1) |
| 4 | Same as #3 but Phase 0 only (env never set on runtime machine) | ❌ | n/a | Silent fallback to registry → air-gap failure |
| 5 | Mirror URL missing trailing slash | ✅ | ✅ | `NewMirrorSource` normalizes trailing `/` |
| 6 | `ftp://…` or non-http(s) mirror URL | ❌ | ❌ | Rejected at source construction |

**Example that works (Phase 1):**

```bash
pulumi package add terraform-provider -- \
  registry.terraform.io/hashicorp/random 3.6.0 \
  --provider-mirror https://myartifactory.example.com/artifactory/api/terraform/providers/

# Later, other machine, no env, empty cache:
pulumi up   # still uses mirror via embedded Value
```

**Mirror HTTP paths hit:**

```text
GET …/providers/registry.terraform.io/hashicorp/random/index.json
GET …/providers/registry.terraform.io/hashicorp/random/3.6.0.json
```

### 8A.2 Terraform vs OpenTofu addresses

| # | What you do | Mirror publishes | Result | Notes |
|---|-------------|------------------|--------|-------|
| 7 | `hashicorp/random` + mirror | only `registry.terraform.io/…` | ❌ | Bare name → `registry.opentofu.org/hashicorp/random` → wrong path / 404 |
| 8 | `hashicorp/random` + mirror | `registry.opentofu.org/…` | ✅ | Matches default host |
| 9 | `registry.terraform.io/hashicorp/random` + mirror | `registry.terraform.io/…` | ✅ | Explicit TF host — recommended when mirror is TF-oriented |
| 10 | `registry.opentofu.org/hashicorp/random` + mirror | `registry.opentofu.org/…` | ✅ | Explicit OpenTofu host |
| 11 | One mirror base serves **both** host prefixes | both layouts present | ✅ | Same `MIRROR`, different `{hostname}/…` paths — supported |

**Does not work (host mismatch):**

```bash
# Mirror only has Terraform-registry layout
pulumi package add terraform-provider -- \
  hashicorp/random 3.6.0 \
  --provider-mirror "$MIRROR"
# → looks up …/registry.opentofu.org/hashicorp/random/… → 404
```

**Works (qualify the host):**

```bash
pulumi package add terraform-provider -- \
  registry.terraform.io/hashicorp/random 3.6.0 \
  --provider-mirror "$MIRROR"
```

### 8A.3 Precedence: flag vs env vs registry

| # | Flag | Env | Effective source | P1 |
|---|------|-----|------------------|----|
| 12 | `--provider-mirror A` | `MIRROR=B` | **A** | ✅ |
| 13 | unset | `MIRROR=B` | **B** | ✅ |
| 14 | unset | unset | Registry disco | ✅ |
| 15 | `--provider-mirror ""` (empty) | `MIRROR=B` | Treat as unset → **B** | ✅ | Empty string must mean unset |

### 8A.4 Same Artifactory host: public mirror + private providers

See also §10A.

| # | Setup | Public TF provider | Private `myartifactory…/myorg/custom` | P1 |
|---|-------|--------------------|----------------------------------------|----|
| 16 | `--provider-mirror` **only** on the public package; private package has **no** mirror flag | ✅ via mirror | ✅ via registry disco on `myartifactory…` | ✅ **Recommended** |
| 17 | Global `PULUMI_TF_NETWORK_MIRROR_URL=MIRROR` for the whole process | ✅ | ❌ / ⚠️ | Private addr forced through mirror path `…/providers/myartifactory.example.com/myorg/custom/…` — 404 unless that layout exists |
| 18 | `--provider-mirror` also set on the private package | ✅ | ❌ / ⚠️ | Same as #17 for that package |
| 19 | Private providers also published **in mirror layout** under hostname `myartifactory.example.com` | ✅ | ✅ even with global env | Possible but uncommon; don't assume |
| 20 | Phase 3: mirror `include = registry.terraform.io/*`, private host uses direct | ✅ | ✅ | 🔜 Proper TF-like split |

**Works (Phase 1 mixed project):**

```yaml
# Pulumi.yaml (illustrative)
packages:
  random:
    source: terraform-provider
    parameters:
      - registry.terraform.io/hashicorp/random
      - 3.6.0
      - --provider-mirror
      - https://myartifactory.example.com/artifactory/api/terraform/providers/
  custom:
    source: terraform-provider
    parameters:
      - myartifactory.example.com/myorg/custom
      - 1.2.3
      # no --provider-mirror → registry protocol against myartifactory.example.com
```

**Does not work (global env + private package, typical Artifactory):**

```bash
export PULUMI_TF_NETWORK_MIRROR_URL=https://myartifactory.example.com/artifactory/api/terraform/providers/
pulumi package add terraform-provider myartifactory.example.com/myorg/custom 1.2.3
# → network-mirror GET …/myartifactory.example.com/myorg/custom/index.json
# → usually 404; private pkgs expect .well-known registry discovery instead
```

### 8A.5 Authentication

| # | Situation | P0/P1 | P2 |
|---|-----------|-------|----|
| 21 | Anonymous-readable mirror | ✅ | ✅ |
| 22 | Mirror requires bearer token; no creds configured | ❌ 401/403 | ❌ until token set |
| 23 | `TF_TOKEN_myartifactory_example_com=…` set in environment | ❌ (ignored) | ✅ |
| 24 | `--provider-mirror-token` in parameters / `Pulumi.yaml` | ❌ by design | ❌ by design — secrets must not be committed |
| 25 | Token only in Pulumi config/state | ❌ | ❌ — use process env / local cred files only |

**Works (Phase 2):**

```bash
export TF_TOKEN_myartifactory_example_com='****'
pulumi package add terraform-provider -- \
  registry.terraform.io/hashicorp/random 3.6.0 \
  --provider-mirror https://myartifactory.example.com/artifactory/api/terraform/providers/
```

### 8A.6 Local providers, cache, and invalid combos

| # | What you do | Result | Notes |
|---|-------------|--------|-------|
| 26 | `./path/to/terraform-provider-foo` (local) | ✅ no mirror | Local path does not download |
| 27 | Local path **and** `--provider-mirror` | ❌ | Phase 1 should error (invalid combination) |
| 28 | Provider already in `PULUMI_DYNAMIC_TF_PLUGIN_CACHE_DIR` matching addr+version | ✅ cache hit | Skips network; can **hide** a bad mirror/env config |
| 29 | Wrong mirror, but cache warm from earlier registry download | ⚠️ appears to work | Clear cache when validating mirror setups |
| 30 | `pulumi package add` with mirror; runtime without `Value.mirror` (Phase 0 only) | ❌ on cold cache | Why Phase 1 exists |

### 8A.7 Out of scope / not expected to work

| # | Expectation | Status | Notes |
|---|-------------|--------|-------|
| 31 | `PULUMI_PLUGIN_DOWNLOAD_URL_OVERRIDES` redirects Terraform provider downloads | ❌ | Only affects Pulumi plugins, not TF provider fetch in the bridge |
| 32 | Encoding the mirror into the provider source string (e.g. fake host paths) | ❌ | Invalid / wrong layer; use `--provider-mirror` or env |
| 33 | Reading `.terraformrc` / `.tofurc` `provider_installation` automatically | ❌ today · 🔜 Phase 4 maybe | Not committed (D7) |
| 34 | `filesystem_mirror` directories | ❌ today · 🔜 Phase 4 | |
| 35 | include/exclude on mirror vs direct | ❌ today · 🔜 Phase 3 | Needed for env-wide mirror + private host (§8A.4 #20) |
| 36 | Static (non-dynamic) bridged providers via this mirror | ❌ | Those ship as Pulumi plugins; different pipeline |

### 8A.8 Quick decision guide

```text
Need air-gap for public TF providers on one CI machine?
  → Phase 0 env var is enough for that machine

Need the project to stay air-gap safe on every clone without env?
  → Phase 1 --provider-mirror (persisted in Value)

Mirror is TF-registry layout?
  → Always pass registry.terraform.io/... (don't rely on bare names)

Same Artifactory host also serves private providers?
  → Put --provider-mirror only on public packages
  → Do NOT set global env on those machines (until Phase 3)

Mirror needs a password?
  → Phase 2 + TF_TOKEN_<host> (never put token in Pulumi.yaml)
```

---

## 9. Feature matrix

| Capability | Tier | Phase | Notes |
|------------|------|-------|-------|
| Single mirror via env | **MUST** | 0 | PR #3463 |
| Single mirror via `--provider-mirror` | **MUST** | 1 | Closes #3334 |
| Persist mirror in `Value` | **MUST** | 1 | Runtime durability |
| Precedence flag > env > default | **MUST** | 1 | |
| TF + OpenTofu host paths on one mirror | **MUST** | 0/1 | Already in `providerURL`; add tests |
| Docs for qualified hosts | **MUST** | 1 | Avoid silent 404 confusion |
| `TF_TOKEN_*` (and optionally netrc) | **SHOULD** | 2 | Enterprise auth; no secrets in yaml |
| Archive hash verification | **SHOULD** | 3 | Populate `Authentication` from mirror hashes |
| include/exclude mirror vs direct | **SHOULD** | 3 | See §12 detailed design |
| Multiple ordered mirrors | **LATER** | 4+ | Needs richer config model |
| `filesystem_mirror` | **LATER** | 4+ | Vendored search helpers exist |
| Parse `.terraformrc` / `.tofurc` | **LATER** | 4+ | Not committed (D7) |
| `dev_overrides` | **WON'T** | — | Out of scope |

---

## 10. Phased implementation plan

### Phase 0 — Env-var mirror (current PR #3463)

**Status:** Implemented on `feat/proxy-provider`; rebase onto `main` done.

**Files:**

- `pkg/vendored/opentofu/getproviders/mirror_source.go` (+ tests)
- `dynamic/internal/shim/run/loader.go` (`envNetworkMirror`, `getProviderFromMirror`)
- `dynamic/internal/shim/run/loader_mirror_test.go`
- `dynamic/README.md`

**Success criteria:**

- [x] Env var selects mirror path and skips disco
- [x] Unit tests for `MirrorSource` and loader wiring
- [ ] Optional hardening before merge: explicit test that both `registry.terraform.io` and `registry.opentofu.org` addresses hit the expected mirror paths
- [ ] Link related issues in PR description (`#106`, optionally “partial toward #3334”)

**Out of scope for Phase 0:** parameterization flag, `Value` persistence, auth.

---

### Phase 1 — Durable `--provider-mirror` (follow-up PR)

**Depends on:** Phase 0 merged (or stacked on top of it).

**Files to touch:**

| File | Change |
|------|--------|
| `dynamic/parameterize/args.go` | `--provider-mirror`; `Args.Mirror` |
| `dynamic/parameterize/value.go` | `Value.Mirror`; `IntoArgs` round-trip |
| `dynamic/parameterize/*_test.go` | Parse / marshal / IntoArgs tests |
| `dynamic/main.go` | Copy `Mirror` into `Value`; pass into `getProvider` |
| `dynamic/internal/shim/run/loader.go` | Precedence resolution API |
| `dynamic/README.md` | User docs + host qualification + future include/exclude note |
| This design doc | Link from README if useful |

**Implementation sketch:**

1. Add failing tests for:
   - cobra parsing of `--provider-mirror`
   - `Value` JSON round-trip including `mirror`
   - loader precedence (flag beats env)
   - **regression:** `ParameterizeValue` path with env unset still uses embedded mirror
2. Implement flag + model fields.
3. Thread mirror through `getProvider` / `NamedProvider`.
4. Update docs with examples for TF-hosted and OpenTofu-hosted addresses.
5. Document Phase 3 include/exclude as future work (see §12).

**Success criteria:**

- [ ] `package add … --provider-mirror URL` writes parameters correctly
- [ ] Generated / embedded `Value` contains `mirror`
- [ ] Fresh cache + no env → runtime still uses mirror
- [ ] Flag overrides env when both set
- [ ] Local path + `--provider-mirror` errors clearly
- [ ] README documents host qualification

**Issue linking:** `Fixes #3334` (and references Phase 0 / #106 as related).

---

### Phase 2 — Authenticated mirrors

**Goal:** Enterprise mirrors that return 401/403 without a bearer token work.

**Proposed mechanism (aligned with D2):**

1. When performing mirror HTTP requests, resolve credentials for the **mirror request host** using Terraform-compatible `TF_TOKEN_<host>` encoding.
2. Attach `Authorization: Bearer <token>` when present.
3. Optionally consult netrc as a secondary source (if low-cost and consistent with vendored HTTP helpers).

**Non-goals for Phase 2:**

- `--provider-mirror-token` flag (would leak into `Pulumi.yaml`)
- Storing tokens in parameterized `Value`

**Success criteria:**

- [ ] Authenticated mirror index/version/archive requests succeed with `TF_TOKEN_*`
- [ ] Missing token yields actionable 401/403 errors
- [ ] Docs show Artifactory-oriented example **without** putting secrets in yaml

**Open detail to finalize in Phase 2 design note:** whether tokens are keyed by mirror hostname only, or also by provider registry hostname for any absolute archive URLs on a different host.

---

### Phase 3 — Hardening toward `provider_installation`

Two independent improvements:

1. **Hash verification:** when mirror `version.json` includes hashes, populate package authentication instead of `Authentication: nil`.
2. **include/exclude routing:** see §12.

Either can ship without the other.

---

### Phase 4 — Optional native-config parity

Only if users still need it after Phases 1–3:

- `filesystem_mirror` directories
- Multiple ordered installation methods
- Parsing `.terraformrc` / `.tofurc` `provider_installation`

This phase is **explicitly not committed** (D7).

---

## 10A. Use case: same host as mirror *and* private registry

### 10A.1 Is this a real use case?

**Yes.** Enterprise Artifactory (and similar) setups commonly put both on one hostname, for example `myartifactory.example.com`:

| Role | Typical mechanism | Example |
|------|-------------------|---------|
| **Network mirror** of public registries | Network mirror protocol under a mirror base URL | `https://myartifactory…/api/terraform/…/providers/` serving `registry.terraform.io/…` and/or `registry.opentofu.org/…` paths |
| **Private / custom Terraform providers** | Provider **registry** protocol (`.well-known` + registry APIs) for host `myartifactory…` | Provider address `myartifactory.example.com/myorg/myprovider` |

These are **different protocols** and usually **different repository paths**, even when they share a DNS name. That is a supported Terraform/OpenTofu pattern (`provider_installation` with `network_mirror` for public addresses and `direct` for the private hostname).

### 10A.2 Do they conflict under this design?

**They can — depending on how the mirror is applied.**

Recall: when a mirror URL is selected, the bridge builds:

```text
{mirrorBase}/{providerHostname}/{namespace}/{type}/index.json
```

So for a custom provider `myartifactory.example.com/myorg/myprovider` with mirror base `https://myartifactory.example.com/api/terraform/…/providers/`, Phase 1 would request:

```text
https://myartifactory.example.com/api/terraform/…/providers/myartifactory.example.com/myorg/myprovider/index.json
```

That only works if the mirror repo **also** publishes private providers in network-mirror layout under the Artifactory hostname prefix. Many setups do **not**: private packages live on the registry protocol for `myartifactory.example.com`, while the mirror repo only contains `registry.terraform.io/…` (and maybe `registry.opentofu.org/…`).

| Config style | Public providers via mirror | Custom provider on same host | Conflict? |
|--------------|----------------------------|------------------------------|-----------|
| **Per-package `--provider-mirror` only on public packages** | Yes | No flag → registry disco to `myartifactory…` | **No** (preferred Phase 1 pattern) |
| **Global `PULUMI_TF_NETWORK_MIRROR_URL`** | Yes | Also forced through mirror path | **Yes**, unless private pkgs exist in mirror layout |
| **`--provider-mirror` on the custom package too** | n/a | Forced through mirror path | **Yes**, same caveat |
| **Phase 3 include/exclude** | Mirror only `registry.terraform.io/*` (etc.) | `direct` for `myartifactory…/*` | **No** (TF-like) |

### 10A.3 Phase 1 guidance (no include/exclude yet)

For mixed Artifactory hosts:

1. Prefer **per-package** `--provider-mirror` on packages that come from public registries.
2. Add internal providers **without** `--provider-mirror`, using the fully qualified private address:
   ```bash
   pulumi package add terraform-provider -- \
     myartifactory.example.com/myorg/myprovider 1.2.3
   ```
3. Avoid a process-wide `PULUMI_TF_NETWORK_MIRROR_URL` on machines that also install private-registry providers — or accept that those private providers must be present in the mirror layout / cache.
4. If the org later needs “env var for everything public, but never for `myartifactory…`”, that is exactly **Phase 3 include/exclude** (§12).

### 10A.4 Implications for the roadmap

This use case **strengthens** the rationale for:

- Phase 1 per-package flag (finer than env-only)
- Documenting the Phase 1 limitation (global env is blunt)
- Keeping §12 include/exclude as an explicit follow-up, not an afterthought

It does **not** block Phase 0/1 if users follow the per-package pattern above.

---

## 11. Security and operational considerations

### 11.1 Secrets

- Mirror **URLs** may be committed.
- Mirror **credentials** must remain client-local (`TF_TOKEN_*`, future netrc).
- Never add token fields to `parameters` / `Value`.

### 11.2 Trust

- Using a network mirror means trusting that mirror as a distribution source (same as Terraform).
- Phase 0/1 intentionally skip hash verification (D6).
- Phase 3 should restore verification when hashes are present.

### 11.3 Shared plugin cache

Cache keying is by provider address + version, not by download source. Consequences:

- A provider cached from the public registry can mask a broken mirror config.
- A provider cached from a mirror can mask a missing env var in Phase 0-only setups.

Mitigations:

- Document cache location (`PULUMI_DYNAMIC_TF_PLUGIN_CACHE_DIR`, default under Pulumi home).
- Phase 1 durability reduces surprise for flag users.
- Tests for Phase 1 must clear cache and unset env.

### 11.4 Error UX

Prefer actionable errors:

- Mirror 404 for `registry.opentofu.org/...` when user intended Terraform layout → hint to qualify `registry.terraform.io/...`
- Mirror 401 → hint to set `TF_TOKEN_<mirror-host>` (Phase 2)
- Invalid `--provider-mirror` URL scheme → reject at parse/source construction (already partly in `NewMirrorSource`)

---

## 12. Future detail: include / exclude routing (Phase 3)

This section is a **design placeholder** so #3334-style `provider_installation` expectations have a documented target. **Not in Phase 1.**

### 12.1 Motivation

Terraform CLI config can express:

```hcl
provider_installation {
  network_mirror {
    url     = "https://mirror.example/providers/"
    include = ["registry.terraform.io/hashicorp/*"]
  }
  direct {
    exclude = ["registry.terraform.io/hashicorp/*"]
  }
}
```

Some orgs mirror only a subset of providers and still need direct registry access for others. Phase 1’s “all or nothing” mirror is insufficient for that.

### 12.2 Proposed Pulumi shape (strawman)

Keep bridge-only flags, still opaque to the CLI:

```bash
pulumi package add terraform-provider -- \
  registry.terraform.io/hashicorp/random 3.6.0 \
  --provider-mirror https://mirror.example/providers/ \
  --provider-mirror-include 'registry.terraform.io/hashicorp/*'
```

Or exclude-from-mirror (fall back to direct):

```bash
  --provider-mirror https://mirror.example/providers/ \
  --provider-mirror-exclude 'registry.opentofu.org/*'
```

Persisted fields on `Value` (illustrative):

```go
Mirror        string
MirrorInclude []string `json:"mirrorInclude,omitempty"`
MirrorExclude []string `json:"mirrorExclude,omitempty"`
```

Matching should use provider source address patterns familiar to TF (`hostname/namespace/type` globs). Exact glob syntax should follow Terraform’s `include` / `exclude` semantics where practical.

### 12.3 Resolution algorithm (strawman)

```text
if no mirror URL → registry path
else if address matches exclude → registry path
else if include list non-empty and address does not match include → registry path
else → mirror path
```

Env var remains a global default URL without include/exclude unless we later add companion env vars (not recommended; prefer parameters for selective routing).

### 12.4 Why defer

- Most air-gap setups want **everything** mirrored (Phase 1).
- Glob semantics and testing cost are non-trivial.
- Avoid blocking durable mirror URL support on routing complexity.

---

## 13. Testing strategy

### 13.1 Unit tests (required per phase)

**Phase 0 (existing):**

- `MirrorSource` version listing, package meta, relative/absolute archive URLs, URL validation
- Loader uses env when set

**Phase 1 (new):**

- Args parsing: positional + `--provider-mirror`
- Reject local path + mirror
- `Value` marshal/unmarshal/`IntoArgs` preserves `mirror`
- Precedence table tests (flag/env/default)
- End-to-end-ish test: build `ParameterizeValue` bytes with mirror, unset env, assert mirror HTTP is used (httptest)

**Phase 2:**

- Requests include bearer token when `TF_TOKEN_*` set
- 401 mapping / error text

**Phase 3:**

- Hash mismatch fails install
- include/exclude matrix

### 13.2 Manual / integration validation

Recommended local checks (air-gap simulation):

1. Block DNS or use a fake registry host that fails `.well-known`.
2. Run a local httptest implementing mirror protocol fixtures.
3. Validate Phase 1 success criterion (no env, empty cache, runtime path).

---

## 14. Documentation plan

| Doc | Content |
|-----|---------|
| `dynamic/README.md` | User-facing mirror setup (env + flag), host qualification, auth via `TF_TOKEN_*`, cache notes; link or adapt §8A examples |
| This spec | Design / discussion source of truth (especially §8A works/doesn't) |
| PR descriptions | Phase mapping; issue links |
| Future: website docs | Once Phase 1 ships, consider pulumi.com docs for `terraform-provider` air-gap |

Suggested README outline addition:

1. What a network mirror is (link protocol docs)
2. Env var quick start
3. Per-package `--provider-mirror` (preferred for projects)
4. Terraform vs OpenTofu addresses
5. Authentication (`TF_TOKEN_*`)
6. Limitations (no include/exclude yet; no `.terraformrc` parsing)
7. Roadmap pointer to this spec §9 / §12

---

## 15. Risks and mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| Mirror only in env, not in `Value` | Runtime air-gap failure | Phase 1 mandatory `Value.Mirror` + regression test |
| Default OpenTofu host vs TF-only mirror | Confusing 404s | Docs + error hints; require explicit TF host |
| Secrets in parameters | Credential leak via git | Forbid token flags; `TF_TOKEN_*` only |
| Cache hides misconfig | False confidence | Document cache; tests clear cache |
| Marketing “enterprise ready” before auth | User frustration | Phase 2 before claiming authenticated Artifactory support |
| Scope creep into full `.terraformrc` | Slow delivery | D7; Phase 4 uncommitted |
| Absolute archive URLs on another host | Auth host mismatch | Phase 2 design note; follow redirects carefully |
| Same host = mirror + private registry | Global env forces private addrs through mirror protocol → 404 | Prefer per-package flag; Phase 3 include/exclude; see §10A |

---

## 16. Alternatives considered

### 16.1 Env-only forever

Rejected as the sole solution: not reproducible across machines; does not match iwahbe’s #3334 guidance.

### 16.2 Put mirror only in `Pulumi.yaml` parameters, not `Value`

Rejected: runtime parameterization does not re-read raw YAML parameters; it uses embedded `Value`.

### 16.3 Encode mirror into the provider source string

Rejected: invalid / confusing addresses (see #3334 discussion); breaks address validation; mixes distribution channel with identity.

### 16.4 Full `.terraformrc` parser first

Rejected for MVP: large surface, overlaps poorly with Pulumi’s package parameterization model, delays the durable fix.

### 16.5 CLI-first `--provider-mirror` in `pulumi package add`

Unnecessary: CLI already stores opaque args. Bridge-owned flag keeps semantics next to download logic.

---

## 17. Rollout and issue tracking

| Deliverable | Tracking |
|-------------|----------|
| Phase 0 | PR #3463 → helps #106 |
| Phase 1 | New PR based on #3463 → **Fixes #3334** |
| Phase 2+ | Separate issues/PRs referencing this spec |

Suggested PR #3463 description note (when updating, if desired):

> This PR implements the environment-variable half of network mirror support (Phase 0 in `docs/superpowers/specs/2026-07-23-network-mirror-design.md`). Per-package `--provider-mirror` (Phase 1) will follow and is what fully addresses #3334.

---

## 18. Appendix A — Key file reference

| Path | Role |
|------|------|
| `dynamic/main.go` | `XParamaterize`, `getProvider` |
| `dynamic/parameterize/args.go` | CLI/args parsing |
| `dynamic/parameterize/value.go` | SDK-embedded parameterization |
| `dynamic/info.go` | Embeds `Value` into package schema |
| `dynamic/internal/shim/run/loader.go` | Download + run TF providers |
| `pkg/vendored/opentofu/getproviders/mirror_source.go` | Network mirror protocol client |
| Pulumi CLI `pkg/cmd/pulumi/packagecmd/package_add.go` | Opaque `parameters` persistence (no change expected) |

---

## 19. Appendix B — Protocol references

- Terraform: [Provider Network Mirror Protocol](https://developer.hashicorp.com/terraform/internals/provider-network-mirror-protocol)
- OpenTofu: [Provider Network Mirror Protocol](https://opentofu.org/docs/internals/provider-network-mirror-protocol/)
- OpenTofu CLI: [`provider_installation`](https://opentofu.org/docs/cli/config/config-file/#provider-installation)
- Terraform CLI: [Provider Installation](https://developer.hashicorp.com/terraform/cli/config/config-file#provider-installation)
- `TF_TOKEN_*` credential env vars: OpenTofu/Terraform CLI config credentials docs

---

## 20. Appendix C — Decision log summary

1. Persist mirror in `Value` (runtime durability).
2. Auth via `TF_TOKEN_*` only; never parameterization.
3. Keep OpenTofu default host; require explicit TF host when needed.
4. v1 mirror applies to all remote downloads.
5. Single `--provider-mirror` string.
6. Trust mirror hashes in v1.
7. Flag + env are permanent API; `.terraformrc` uncommitted later.
8. Precedence: flag > env > registry.
9. Phase 0 then Phase 1 as separate PRs.

---

## 21. Approval

- [ ] Design approved for Phase 0 merge criteria
- [ ] Design approved to implement Phase 1 against this spec
- [ ] Phase 2+ accepted as roadmap (not blocking Phase 1)

**Reviewers:** please comment on §4 decisions, §7 data model, and §12 future routing if those should change before Phase 1 starts.
