# Maintainer discussion: network mirror support for dynamic `terraform-provider`

**Status:** Questions for Pulumi maintainers  
**Design draft:** [`docs/superpowers/specs/2026-07-23-network-mirror-design.md`](./2026-07-23-network-mirror-design.md)  
**Related:** [#3334](https://github.com/pulumi/pulumi-terraform-bridge/issues/3334), [pulumi-terraform-provider#106](https://github.com/pulumi/pulumi-terraform-provider/issues/106), [#3463](https://github.com/pulumi/pulumi-terraform-bridge/pull/3463)  

## Why we’re asking

Air-gapped and enterprise users need the dynamic bridge to download Terraform providers via the [network mirror protocol](https://developer.hashicorp.com/terraform/internals/provider-network-mirror-protocol) without calling registry `.well-known` discovery.

Terraform exposes this through `.terraformrc` / `.tofurc` `provider_installation { network_mirror { … } }`. Pulumi has no equivalent today for **Terraform** provider binaries ( `PULUMI_PLUGIN_*` only affects Pulumi plugins).

We’ve drafted a Pulumi-native design and would like maintainer preference on several forks before implementing further. A discussion PR will carry the design doc (and later an implementation plan).

Under each question we note our current lean. Please push back freely—including on options we dismissed.

---

## 1. Configuration surfaces

Terraform configures mirrors only in CLI config files. Pulumi’s closest pattern for downloads is environment overrides (for example `PULUMI_PLUGIN_HOST_OVERRIDES`), not reading `.terraformrc`.

### Q1. Primary config approach

Which approach do you prefer?

| Option | Description |
|--------|-------------|
| **A** | Pulumi-native only: `PULUMI_TF_NETWORK_MIRROR_OVERRIDES` + per-package `--provider-mirror` |
| **B** | Also parse `.terraformrc` / `.tofurc` / `TF_CLI_CONFIG_FILE` for drop-in Terraform parity |
| **C** | Both, with documented precedence |

**Our lean:** **A** — avoid dual sources of truth; match how Pulumi already handles plugin download routing.

### Q2. Environment variable naming

If we ship an overrides env var, is this name acceptable?

- `PULUMI_TF_NETWORK_MIRROR_OVERRIDES`

Or would you prefer something closer to plugin naming, such as `PULUMI_TF_PROVIDER_HOST_OVERRIDES`?

**Our lean:** `PULUMI_TF_NETWORK_MIRROR_OVERRIDES`, because this is **network-mirror protocol + skip `.well-known`**, not a transparent HTTP host rewrite like `PULUMI_PLUGIN_HOST_OVERRIDES`.

### Q3. Single catch-all URL env

We currently plan **not** to ship `PULUMI_TF_NETWORK_MIRROR_URL`. Catch-all would be:

```bash
export PULUMI_TF_NETWORK_MIRROR_OVERRIDES='*=https://mirror.example/providers/'
```

Is that enough, or do you still want a dedicated one-URL env for short CI one-liners?

**Our lean:** overrides only — one env is enough.

---

## 2. Per-package `--provider-mirror`

Inspired by maintainer feedback on the draft PR: store mirror config in package parameters, for example:

```bash
pulumi package add terraform-provider -- \
  registry.terraform.io/hashicorp/random 3.6.0 \
  --provider-mirror https://mirror.example/providers/
```

→ lands in `Pulumi.yaml` `packages.*.parameters`.

We also plan to persist the mirror URL in the SDK-embedded parameterized **`Value`**, so runtime `preview` / `up` still works on a cold cache without the env var.

### Q4. Persist mirror in parameterized `Value`?

Do you agree the mirror URL must round-trip through the embedded `Value`, not only `Pulumi.yaml` parameters?

**Our lean:** **yes** — otherwise env-only setups fail in air-gap when the cache is empty at runtime.

### Q5. Precedence

Is this precedence acceptable?

```text
--provider-mirror  >  PULUMI_TF_NETWORK_MIRROR_OVERRIDES  >  registry discovery
```

**Our lean:** yes.

### Q6. First-class `Pulumi.yaml` field?

Would you rather a first-class package field (for example `providerMirror:`) instead of an opaque parameterization flag in `parameters`?

**Our lean:** keep the flag in `parameters` (CLI already opaque-stores args; bridge-only change). Revisit a first-class field only if the CLI/package schema wants it.

---

## 3. Routing and Terraform `include` / `exclude` parity

Proposed overrides shape (one env). **Phase 1** ships exact-host keys, literal `*`, and `!pattern` deny. TF path globs (`host/ns/*`) are **Phase 4**.

```bash
# Phase 1 — include host → mirror protocol, skip .well-known
registry.terraform.io=https://mirror.example/providers/

# Phase 1 — deny host → direct (evaluated in code; Go RE2 has no negative lookahead)
!myartifactory.example.com

# Phase 1 — catch-all
*=https://mirror.example/providers/

# Phase 4 — path glob include / deny
# registry.terraform.io/hashicorp/*=https://mirror.example/providers/
# !registry.terraform.io/evil/*
```

No match → direct registry discovery.

Patterns match the **regaddr-resolved** address `hostname/namespace/type` (so bare `hashicorp/random` is `registry.opentofu.org/hashicorp/random`).

### Q7. Pattern syntax (Phase 4 richness)

Phase 1 proposes exact-host + `*` + `!`. For **Phase 4**, prefer override keys as:

| Option | Description |
|--------|-------------|
| **A** | Terraform-style globs (`registry.terraform.io/hashicorp/*`), including TF shorthands (`hashicorp/*` → `registry.terraform.io/hashicorp/*`) |
| **B** | RE2 regex only |
| **C** | Both |

**Our lean:** **A** or **C**, with TF glob shorthands if we claim glob parity.

### Q8. Match order

When multiple positive patterns match, prefer:

| Option | Description |
|--------|-------------|
| **A** | First-match wins (document: put specific patterns before `*`) |
| **B** | Most-specific wins |

**Our lean:** **A** (simple, predictable).

### Q9. Deny syntax

Is `!pattern` in the same overrides string acceptable for Terraform `exclude`-style behavior?

We plan to ship `!` deny in **Phase 1** (exact-host / `*` forms). Path-glob denies wait for Phase 4 with globs.

Alternatives we considered: a second env var, or Perl-style negative lookahead in regex (rejected — Go RE2 does not support lookaround).

**Our lean:** `!pattern` in the same env (Phase 1).

### Q10. Multi-method “newest version”

Terraform can query multiple matching install methods and pick the **newest** version across them. We would pick **one** source (flag or first override match) and stop.

Is that acceptable for v1?

**Our lean:** yes — sufficient for air-gap / Artifactory; document the difference.

---

## 4. Same host for mirror and private registry

A common Artifactory setup uses one hostname for:

1. a **network mirror** of public registries, and  
2. a **private Terraform registry** for internal providers (`myartifactory.example.com/myorg/pkg`).

### Q11. No automatic same-host skip

We are **not** planning magic that silently skips the mirror when `provider.Hostname == mirrorURL.Host`.

Users should configure explicitly, for example:

```bash
# scoped (recommended)
PULUMI_TF_NETWORK_MIRROR_OVERRIDES='registry.terraform.io=https://myartifactory…/providers/'

# or catch-all with deny (Phase 1: deny exact host)
PULUMI_TF_NETWORK_MIRROR_OVERRIDES='*=https://myartifactory…/providers/,!myartifactory.example.com'
```

If they use bare `*=…` and also install private providers on that host, failure is treated as user misconfiguration.

Do you prefer this explicit model, or do you want automatic same-host rescue (warn or silent skip) anyway?

**Our lean:** explicit only — no same-host magic.

---

## 5. Authentication and secrets

### Q12. `TF_TOKEN_*`

Should mirror/registry HTTP auth use Terraform-compatible `TF_TOKEN_<host>` environment variables?

**Our lean:** yes (Phase 3).

### Q13. Pulumi credentials file

Later, should we allow storing tokens in `~/.pulumi/credentials.json` under something like `terraformProviderCredentials` (precedence: env > file), similar to how `PULUMI_ACCESS_TOKEN` relates to stored cloud credentials?

**Our lean:** worth considering for interactive use; not required for the first auth milestone. Tokens must never live in `Pulumi.yaml` or stack config.

### Q14. No token parameterization flag

Please confirm we should **not** support `--provider-mirror-token` (or similar) in package parameters, because it would be committed to git.

**Our lean:** never put download tokens in parameters / `Value`.

---

## 6. Deferred scope — should any come back earlier?

### Q15. Priority of deferred items

Which of these should move earlier than we planned?

| Item | Current plan |
|------|----------------|
| `filesystem_mirror` (local directory layout) | Later |
| Parsing `.terraformrc` / `.tofurc` | Out of primary scope |
| OpenTofu `oci_mirror` | Out of scope unless demanded |
| OpenTofu `download_retry_count` / `trust_all_hashes` | Out of scope |
| Hash verification of mirror archives in v1 | Later (trust mirror initially) |
| Implied Terraform plugin dirs (`~/.terraform.d/plugins`, etc.) | Out of scope |

### Q16. `dev_overrides`

Leave Terraform-style `dev_overrides` out for dynamic providers?

**Our lean:** out of scope.

---

## 7. Delivery and issue tracking

### Q17. Phased delivery

Is this phasing acceptable?

1. **Phase 1:** `MirrorSource` + `PULUMI_TF_NETWORK_MIRROR_OVERRIDES` (exact-host / `*` / `!pattern` deny)
2. **Phase 2:** `--provider-mirror` + persist in `Value` → address #3334
3. **Phase 3:** Auth (`TF_TOKEN_*`, maybe credentials store)
4. **Phase 4+:** TF path globs / optional RE2, hash verification, optional filesystem mirror

### Q18. Closing #3334

Should #3334 be closed when durable `--provider-mirror` + Phase 1 overrides land, or only once Phase 4 globs land?

**Our lean:** close (or largely close) after Phase 2; Phase 1 already covers env-based exclude via `!`; keep follow-ups for glob polish.

### Q19. “Behavioral parity” framing

Any objection to documenting this as **behavioral** Terraform parity (same outcomes) rather than file-compatible `.terraformrc` support?

**Our lean:** behavioral parity is the right product framing.

---

## 8. Alignment with other Pulumi work

### Q20. Related work in `pulumi/pulumi`

Is there existing or planned work (package add, credentials store, plugin host overrides, docs) we should align with before implementing bridge-only pieces?

### Q21. Documentation venue

For the first user-facing docs, is `dynamic/README.md` enough, or should pulumi.com docs land in the same change set?

**Our lean:** README first; website docs as a follow-up unless you prefer otherwise.

---

## Current lean (summary)

| Topic | Lean |
|-------|------|
| Surfaces | `OVERRIDES` + `--provider-mirror`; no `.terraformrc`; no single `MIRROR_URL` |
| Phase 1 grammar | Exact-host, `*`, `!pattern` deny |
| Durability | Mirror URL in parameterized `Value` (Phase 2) |
| Routing | Match resolved `host/ns/type`; flag > overrides; no same-host auto-skip |
| Auth | `TF_TOKEN_*` first; optional credentials.json later |
| Later | TF path globs (Phase 4); filesystem mirror (Phase 5); no rc parse |

We're happy to revise the design from your answers before more implementation. Thank you!
