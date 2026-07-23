# Phase 2 — `--provider-mirror` + Value Persistence Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Per-package durable mirror via `--provider-mirror`, stored in CLI parameters and SDK-embedded `Value`, so cold-cache runtime still uses the mirror without env. Closes [#3334](https://github.com/pulumi/pulumi-terraform-bridge/issues/3334).

**Architecture:** Extend `parameterize.Args` / `Value` with optional `Mirror` URL. Cobra flag on Parameterize. Thread URL into `NamedProvider` / `getProviderServer`. Precedence: **flag/`Value` mirror > `PULUMI_TF_NETWORK_MIRROR_OVERRIDES` > registry**. Flag wins even over a matching `!deny` in OVERRIDES.

**Tech Stack:** Same as Phase 1. Depends on Phase 1 (`MirrorSource`, OVERRIDES, `TF_TOKEN_*`) already merged.

**Spec:** [design.md](./design.md) §10 Phase 2. **Questions:** [maintainer-questions.md](./maintainer-questions.md).

## Global Constraints

- Bridge-only flag storage: CLI already opaque-stores package parameters — no `pulumi/pulumi` CLI schema change required.
- Never put tokens in `Value` / parameters / `Pulumi.yaml`.
- Local path + `--provider-mirror` → clear error.
- Empty / whitespace mirror → treat as unset.
- Normalize trailing `/` via `NewMirrorSource`.
- `http` and `https` allowed (same as Phase 1).

---

## File map

| File | Responsibility |
|------|----------------|
| `dynamic/parameterize/args.go` | `--provider-mirror`; `Args.Mirror` |
| `dynamic/parameterize/args_test.go` | Flag parse / reject local+mirror |
| `dynamic/parameterize/value.go` | `Mirror` JSON field; `IntoArgs` |
| `dynamic/parameterize/value_test.go` | Marshal round-trip |
| `dynamic/main.go` | Copy Mirror Args↔Value; pass to loader |
| `dynamic/internal/shim/run/loader.go` | Accept explicit mirror; precedence |
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

```go
// ParseArgs with ["hashicorp/random", "3.5.1", "--provider-mirror", "https://m.example/p/"]
// → Args.Remote set, Args.Mirror == "https://m.example/p/"

// ParseArgs with ["./bin/terraform-provider-x", "--provider-mirror", "https://m.example/p/"]
// → error (local + mirror incompatible)
```

- [ ] **Step 2: Add flag**

```go
cmd.Flags().StringVar(&mirror, "provider-mirror", "",
	"Network mirror base URL for downloading this provider (Terraform network_mirror protocol)")
```

Thread `mirror` into `parseArgs`; after building Args:

```go
if args.Local != nil && mirror != "" {
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

- [ ] **Step 1: Failing round-trip test**

```go
v := Value{
	Remote: &RemoteValue{URL: "registry.terraform.io/hashicorp/random", Version: "3.5.1"},
	Mirror: "https://m.example/providers/",
}
b := v.Marshal()
got, err := ParseValue(b)
require.NoError(t, err)
require.Equal(t, "https://m.example/providers/", got.Mirror)
args := got.IntoArgs()
require.Equal(t, got.Mirror, args.Mirror)
```

Autogold update if this package uses golden JSON snapshots (`PULUMI_ACCEPT=1` / existing pattern in `value_test.go`).

- [ ] **Step 2: Implement field + `IntoArgs` copy**

```go
func (p *Value) IntoArgs() Args {
	a := /* existing remote/local mapping */
	a.Mirror = p.Mirror
	return a
}
```

Ensure `main.go` path that builds `Value` from `Args` sets `Mirror: args.Mirror`.

- [ ] **Step 3: PASS + commit**

```bash
git commit -m "feat(dynamic): persist provider-mirror in parameterized Value"
```

---

### Task 3: Thread mirror through main → loader

**Files:**
- Modify: `dynamic/main.go`
- Modify: `dynamic/internal/shim/run/loader.go`
- Modify tests as needed (`dynamic/provider_test.go` optional e2e)

**Interfaces:**

```go
// loader.go
func NamedProvider(ctx context.Context, key, version, mirrorURL string) (Provider, error)

func getProviderServer(
	ctx context.Context, addr addrs.Provider, version getproviders.VersionConstraints,
	registryDisco *disco.Disco, mirrorURL string,
) (Provider, error)
```

Precedence inside `getProviderServer` (cache miss):

```go
var source getproviders.Source
switch {
case strings.TrimSpace(mirrorURL) != "":
	source, err = getproviders.NewMirrorSource(mirrorURL, TFTokenCredentialsFromEnv(), nil)
case /* overrides.Resolve */ :
	source, err = getproviders.NewMirrorSource(overrideURL, TFTokenCredentialsFromEnv(), nil)
default:
	source = getproviders.NewRegistrySource(ctx, registryDisco, nil, getproviders.LocationConfig{})
}
```

Update all `NamedProvider` / `getProvider` call sites in `dynamic/main.go` to pass `args.Mirror`.

- [ ] **Step 1: Unit test precedence**

| Input | Expected source |
|-------|-----------------|
| mirrorURL set + OVERRIDES deny for addr | Mirror (flag wins) |
| mirrorURL empty + OVERRIDES match | Mirror (env) |
| both empty | Registry |

Reuse `selectProviderSource` seam from Phase 1 if present; else extract one.

- [ ] **Step 2: Implement + compile all callers**

```bash
go test ./dynamic/... -count=1
```

- [ ] **Step 3: Commit**

```bash
git commit -m "feat(dynamic): honor --provider-mirror over OVERRIDES"
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

Precedence table: flag > OVERRIDES > registry. Auth still `TF_TOKEN_*`. Cold cache: mirror comes from embedded `Value`.

- [ ] **Step 1: README**
- [ ] **Step 2: Commit**

```bash
git commit -m "docs(dynamic): document --provider-mirror and precedence"
```

PR description: `Fixes #3334`. Reference Phase 1 / #106.

---

## Phase 2 success criteria

- [ ] `package add … --provider-mirror URL` → parameters contain mirror
- [ ] Embedded `Value` has `"mirror":"…"`
- [ ] Cold cache, no OVERRIDES env → still downloads via mirror
- [ ] Flag overrides env including `!deny`
- [ ] Local + flag errors clearly

## Out of scope

credentials.json, hash verification, filesystem mirror, `.terraformrc`, RE2 keys, changing Pulumi CLI package schema.
