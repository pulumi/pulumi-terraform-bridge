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

Terraform configures mirrors only in CLI config files. Pulumi’s closest official pattern for plugin download rewriting is `PULUMI_PLUGIN_DOWNLOAD_URL_OVERRIDES` (Go regexp on URLs) — a different problem than Terraform provider-address globs.

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

Or would you prefer a different name?

**Our lean:** `PULUMI_TF_NETWORK_MIRROR_OVERRIDES`, because this selects the **network-mirror protocol** and skips `.well-known` (not a generic URL rewrite).

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

Proposed overrides shape (one env). **Phase 1** includes exact-host, `*`, TF path globs/shorthands, and `!pattern` deny:

```bash
# include host
registry.terraform.io=https://mirror.example/providers/

# TF glob
registry.terraform.io/hashicorp/*=https://mirror.example/providers/

# deny
!myartifactory.example.com

# catch-all
*=https://mirror.example/providers/
```

No match → direct registry discovery.

Patterns match the **regaddr-resolved** address `hostname/namespace/type` (so bare `hashicorp/random` is `registry.opentofu.org/hashicorp/random`).

### Q7. Pattern language for override keys

Override keys need a match language. Two options people often mix up:

| Option | Description | Precedent |
|--------|-------------|-----------|
| **A** | Terraform-style globs only (`registry.terraform.io/hashicorp/*`, shorthands like `hashicorp/*`, plus exact host / `*` / `!pattern`) | `.terraformrc` `include`/`exclude` |
| **B** | Go/RE2 regexp keys (like `PULUMI_PLUGIN_DOWNLOAD_URL_OVERRIDES`) | Official Pulumi plugin URL overrides |
| **C** | Support both | — |

**Our lean:** **A** only. One language, matches Terraform mental model, avoids `*` meaning different things in glob vs regex. We do **not** plan RE2 keys unless maintainers insist.

### Q8. Match order

When multiple positive patterns match, prefer:

| Option | Description |
|--------|-------------|
| **A** | First-match wins (document: put specific patterns before `*`) |
| **B** | Most-specific wins |

**Our lean:** **A** (simple, predictable).

### Q9. Deny syntax

Is `!pattern` in the same overrides string acceptable for Terraform `exclude`-style behavior (Phase 1)?

Alternatives considered: a second env var, or Perl-style negative lookahead (rejected — Go RE2 has no lookaround).

**Our lean:** `!pattern` in the same env, Phase 1.

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

Should Phase 1 include Terraform-compatible `TF_TOKEN_<host>` for mirror HTTP auth?

**Our lean:** yes — Artifactory-style mirrors usually need it; ship with the overrides feature.

### Q13. Pulumi credentials file

Later (Phase 3), should we allow storing tokens in `~/.pulumi/credentials.json` under something like `terraformProviderCredentials` (precedence: env > file)?

**Our lean:** optional follow-up; not required for Phase 1. Tokens must never live in `Pulumi.yaml` or stack config.

### Q14. No token parameterization flag

Please confirm we should **not** support `--provider-mirror-token` (or similar) in package parameters, because it would be committed to git.

**Our lean:** never put download tokens in parameters / `Value`.

---

## 6. Deferred scope — should any come back earlier?

### Q15. Priority of deferred items

Which of these should move earlier than Phase 3?

| Item | Current plan |
|------|----------------|
| Pulumi `credentials.json` token store | Phase 3 |
| Hash verification of mirror archives | Phase 3 |
| `filesystem_mirror` | Phase 3+ |
| Parsing `.terraformrc` / `.tofurc` | Out of primary scope |
| OpenTofu `oci_mirror` | Out of scope unless demanded |

### Q16. `dev_overrides`

Leave Terraform-style `dev_overrides` out for dynamic providers?

**Our lean:** out of scope.

---

## 7. Delivery and issue tracking

### Q17. Phased delivery

Is this phasing acceptable?

1. **Phase 1 (one PR, multi-commit OK):** `MirrorSource` + `OVERRIDES` (exact-host / `*` / TF globs / `!`) + `TF_TOKEN_*`
2. **Phase 2 (follow-up PR):** `--provider-mirror` + persist in `Value` → address #3334
3. **Phase 3+:** credentials.json, hash verify, filesystem

### Q18. Closing #3334

Should #3334 be closed after Phase 2 (durable flag + Value), given Phase 1 already delivers env-based mirror+exclude+auth?

**Our lean:** yes — close (or largely close) after Phase 2; keep Phase 3 items as follow-ups.

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
| Phase 1 | MirrorSource + OVERRIDES (TF globs + `!`) + `TF_TOKEN_*` — one PR |
| Phase 2 | `--provider-mirror` + `Value` — follow-up PR |
| Routing | Match resolved `host/ns/type`; flag > overrides; no same-host auto-skip |
| Auth | `TF_TOKEN_*` in Phase 1; credentials.json optional Phase 3 |
| Later | Hash verify, filesystem; no rc parse; no RE2 override keys |

We're happy to revise the design from your answers before more implementation. Thank you!
