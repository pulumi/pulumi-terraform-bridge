# Phase 2 — `--provider-mirror` + Value Persistence Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Per-package durable mirror via `--provider-mirror`, stored in CLI parameters and SDK-embedded `Value`, so cold-cache runtime still uses the mirror without env. Closes [#3334](https://github.com/pulumi/pulumi-terraform-bridge/issues/3334).

**Architecture:** Extend `parameterize.Args` / `Value` with optional `Mirror` URL. Cobra flag on Parameterize. Thread URL into loader via Phase 1’s **required** `selectProviderSource(ctx, addr, mirrorURL, disco)`. Precedence: **flag/`Value` mirror > `PULUMI_TF_NETWORK_MIRROR_OVERRIDES` > registry**. Flag wins even over a matching `!deny` in OVERRIDES.

**Tech Stack:** Same as Phase 1. Depends on Phase 1 (`MirrorSource`, OVERRIDES, `TF_TOKEN_*`, `selectProviderSource`) already merged.

**Spec:** [design.md](./design.md) §10 Phase 2. **Questions:** [maintainer-questions.md](./maintainer-questions.md).

## Global Constraints

- Bridge-only flag storage: CLI already opaque-stores package parameters — no `pulumi/pulumi` CLI schema change required.
- Never put tokens in `Value` / parameters / `Pulumi.yaml`.
- Local path + `--provider-mirror` → clear error.
- Empty / whitespace mirror → treat as unset.
- Normalize trailing `/` via `NewMirrorSource`.
- `http` and `https` allowed (same as Phase 1).
- Use Phase 1 `selectProviderSource` — do **not** invent a second routing path.
- **API note:** `NamedProvider` is exported but has a single caller (`dynamic/main.go`). Signature change to add `mirrorURL` is safe; update that one call site.

---

## File map

| File | Responsibility |
|------|----------------|
| `dynamic/parameterize/args.go` | `--provider-mirror`; `Args.Mirror` |
| `dynamic/parameterize/args_test.go` | Flag parse / reject local+mirror / empty |
| `dynamic/parameterize/value.go` | `Mirror` JSON field; `IntoArgs` |
| `dynamic/parameterize/value_test.go` | Marshal round-trip + backward compat |
| `dynamic/main.go` | Copy Mirror Args↔Value; pass to loader |
| `dynamic/internal/shim/run/loader.go` | Pass `mirrorURL` into `selectProviderSource` / `NamedProvider` |
| `dynamic/internal/shim/run/loader_test.go` | Precedence + cold-cache tests |
| `dynamic/README.md` | Document flag + precedence |

---

### Task 1: Args + cobra flag

**Files:**
- Modify: `dynamic/parameterize/args.go`
- Modify: `dynamic/parameterize/args_test.go`

**Interfaces:**
- Produces: `Args.Mirror string` (empty = unset)

```go
type Args struct {
	Remote *RemoteArgs
	Local  *LocalArgs
	Includes []string
	Excludes []string
	ProviderName string
	Mirror string // absolute mirror base URL; empty = unset
}
```

- [ ] **Step 1: Failing tests**

1. `TestParseArgs_ProviderMirrorFlag` — `["hashicorp/random", "3.5.1", "--provider-mirror", "https://m.example/p/"]` → `Args.Mirror` set
2. `TestParseArgs_ProviderMirrorEmpty` — `["hashicorp/random", "3.5.1", "--provider-mirror", ""]` → `Args.Mirror == ""` (unset)
3. `TestParseArgs_ProviderMirrorWhitespace` — whitespace-only → unset after trim
4. `TestParseArgs_LocalPlusMirrorError` — `["./bin/terraform-provider-x", "--provider-mirror", "https://m.example/p/"]` → error
5. `TestMirrorTrailingSlashNormalization` — either Args stores raw URL and `NewMirrorSource` normalizes, or trim/add slash consistently; assert download path works without trailing slash in flag value (may live in loader tests)

- [ ] **Step 2: Add flag**

```go
cmd.Flags().StringVar(&mirror, "provider-mirror", "",
	"Network mirror base URL for downloading this provider (Terraform network_mirror protocol)")
```

```go
if args.Local != nil && strings.TrimSpace(mirror) != "" {
	return Args{}, status.Error(codes.InvalidArgument,
		"--provider-mirror cannot be used with a local provider path")
}
args.Mirror = strings.TrimSpace(mirror)
```

- [ ] **Step 3: PASS + commit**

```bash
go test ./dynamic/parameterize/ -run TestParseArgs -count=1
git commit -m "feat(dynamic): add --provider-mirror parameterization flag"
```

---

### Task 2: Value persistence

**Files:**
- Modify: `dynamic/parameterize/value.go`
- Modify: `dynamic/parameterize/value_test.go`

**Interfaces:**

```go
type Value struct {
	Remote *RemoteValue `json:"remote,omitempty"`
	Local  *LocalValue  `json:"local,omitempty"`
	Includes []string `json:"includes,omitempty"`
	Excludes []string `json:"excludes,omitempty"`
	ProviderName string `json:"providerName,omitempty"`
	Mirror string `json:"mirror,omitempty"`
}
```

- [ ] **Step 1: Failing tests**

1. `TestValueMarshal_WithMirror` — autogold / assert JSON contains `"mirror":"..."`
2. `TestValueUnmarshal_WithMirror` — round-trip preserves mirror
3. `TestValueUnmarshal_WithoutMirror_BackwardCompat` — `{"remote":{"url":"...","version":"..."}}` (no mirror) → `Value.Mirror == ""`
4. `TestValueIntoArgs_PreservesMirror` — `Value.Mirror` → `Args.Mirror`

- [ ] **Step 2: Implement field + `IntoArgs`**

```go
func (p *Value) IntoArgs() Args {
	a := /* existing remote/local mapping */
	a.Mirror = p.Mirror
	return a
}
```

`main.go` when building `Value` from `Args`: set `Mirror: args.Mirror`.

- [ ] **Step 3: PASS + commit**

```bash
go test ./dynamic/parameterize/ -run 'TestValue' -count=1
git commit -m "feat(dynamic): persist provider-mirror in parameterized Value"
```

---

### Task 3: Thread mirror through main → loader

**Files:**
- Modify: `dynamic/main.go`
- Modify: `dynamic/internal/shim/run/loader.go`
- Modify: `dynamic/internal/shim/run/loader_test.go`

**Interfaces:**

```go
// Exported; single caller in dynamic/main.go — signature change is intentional.
func NamedProvider(ctx context.Context, key, version, mirrorURL string) (Provider, error)

func getProviderServer(
	ctx context.Context, addr addrs.Provider, version getproviders.VersionConstraints,
	registryDisco *disco.Disco, mirrorURL string,
) (Provider, error)
```

`getProviderServer` calls:

```go
source, err := selectProviderSource(ctx, addr, mirrorURL, registryDisco)
```

Phase 1 already implemented precedence inside `selectProviderSource` when `mirrorURL` non-empty. Phase 2 only threads the argument.

Update all `NamedProvider` / `getProvider` call sites in `dynamic/main.go` to pass `args.Mirror`.

- [ ] **Step 1: Required precedence tests** (use Phase 1 seam)

| Test | Input | Expected |
|------|-------|----------|
| `TestSelectProviderSource_FlagBeatsDeny` | mirrorURL set + OVERRIDES `!addr` | Mirror at **flag** URL |
| `TestSelectProviderSource_FlagBeatsOverride` | mirrorURL set + OVERRIDES match other URL | Mirror at **flag** URL |
| `TestSelectProviderSource_EmptyFlagFallsToOverride` | mirrorURL `""` + OVERRIDES match | Mirror at **env** URL |
| `TestSelectProviderSource_BothEmpty` | both empty | Registry |

- [ ] **Step 2: Implement + compile all callers**

```bash
go test ./dynamic/... -count=1
```

- [ ] **Step 3: Commit**

```bash
git commit -m "feat(dynamic): honor --provider-mirror over OVERRIDES"
```

---

### Task 3.5: Cold-cache runtime test (REQUIRED)

This is the critical test that proves Phase 2’s reason for existing. **Do not ship Phase 2 without it.**

**Files:**
- Modify: `dynamic/internal/shim/run/loader_test.go` and/or `dynamic/provider_test.go`

- [ ] **Step 1: Write failing cold-cache test** `TestColdCacheRuntime_MirrorFromValue`

Procedure:

1. httptest mirror serving `index.json` + `version.json` (+ optional tiny zip if exercising InstallPackage)
2. Build `parameterize.Value` with `Mirror` = httptest mirror base; remote provider addr/version
3. `t.Setenv("PULUMI_TF_NETWORK_MIRROR_OVERRIDES", "")` / unset
4. `t.Setenv("PULUMI_DYNAMIC_TF_PLUGIN_CACHE_DIR", emptyTempDir)` — cold cache
5. Convert Value → Args (`IntoArgs`) → call `NamedProvider` / `selectProviderSource` with `args.Mirror`
6. Assert `*MirrorSource` (or assert HTTP hit on httptest mirror, **not** registry / not disco `.well-known`)
7. Optionally assert at least one GET to `{mirror}/{host}/{ns}/{type}/index.json`

Minimum acceptable: prove routing uses mirror from Value with **no** OVERRIDES env. Prefer also proving metadata HTTP hit.

- [ ] **Step 2: Make it pass**
- [ ] **Step 3: Commit**

```bash
git commit -m "test(dynamic): cold-cache runtime uses Value.mirror"
```

---

### Task 4: Docs + issue close

**Files:**
- Modify: `dynamic/README.md`

Document:

```bash
pulumi package add terraform-provider hashicorp/random 3.5.1 \
  --provider-mirror https://artifactory.example.com/…/providers/
```

Precedence: flag/`Value` > OVERRIDES > registry (including flag beats `!deny`). Auth still `TF_TOKEN_*`. Cold cache: mirror from embedded `Value`.

- [ ] **Step 1: README**
- [ ] **Step 2: Commit**

```bash
git commit -m "docs(dynamic): document --provider-mirror and precedence"
```

PR description: `Fixes #3334`. Reference Phase 1 / #106.

---

## Phase 2 required test checklist (ship gate)

- [ ] `TestParseArgs_ProviderMirrorFlag`
- [ ] `TestParseArgs_ProviderMirrorEmpty`
- [ ] `TestParseArgs_ProviderMirrorWhitespace`
- [ ] `TestParseArgs_LocalPlusMirrorError`
- [ ] `TestValueMarshal_WithMirror`
- [ ] `TestValueUnmarshal_WithMirror`
- [ ] `TestValueUnmarshal_WithoutMirror_BackwardCompat`
- [ ] `TestValueIntoArgs_PreservesMirror`
- [ ] `TestSelectProviderSource_FlagBeatsDeny`
- [ ] `TestSelectProviderSource_FlagBeatsOverride`
- [ ] `TestSelectProviderSource_EmptyFlagFallsToOverride`
- [ ] `TestSelectProviderSource_BothEmpty`
- [ ] `TestColdCacheRuntime_MirrorFromValue` (**REQUIRED**)
- [ ] Trailing-slash normalization covered (flag or NewMirrorSource)

## Phase 2 success criteria

- [ ] All checklist items green
- [ ] `package add … --provider-mirror URL` → parameters contain mirror
- [ ] Embedded `Value` has `"mirror":"…"`
- [ ] Cold cache, no OVERRIDES env → still uses mirror
- [ ] Flag overrides env including `!deny`
- [ ] Local + flag errors clearly

## Out of scope

credentials.json, hash verification, filesystem mirror, `.terraformrc`, RE2 keys, changing Pulumi CLI package schema.
