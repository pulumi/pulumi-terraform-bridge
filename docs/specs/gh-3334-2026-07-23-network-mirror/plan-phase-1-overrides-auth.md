# Phase 1 — MirrorSource + OVERRIDES + TF_TOKEN_* Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Machine-side Terraform network-mirror routing: download dynamic TF providers via `PULUMI_TF_NETWORK_MIRROR_OVERRIDES` (TF globs + `!deny`) and authenticate with `TF_TOKEN_<host>`.

**Architecture:** Add `MirrorSource` implementing `getproviders.Source` (HTTP mirror protocol, skip disco). Parse OVERRIDES into ordered deny/positive rules; match on regaddr-resolved `hostname/namespace/type`. Wire in `dynamic/internal/shim/run/loader.go` before `NewRegistrySource`. Attach bearer tokens from `TF_TOKEN_*` on mirror HTTP.

**Tech Stack:** Go; vendored OpenTofu `getproviders`; `github.com/opentofu/svchost` / `svcauth`; `httptest` for mirror stubs; existing `loader_test.go` patterns.

**Spec:** [design.md](./design.md) §10 Phase 1, §22.4 grammar. **Questions:** [maintainer-questions.md](./maintainer-questions.md) (esp. Q7 — TF globs only).

## Global Constraints

- Pattern language = **Terraform globs only** (exact host, `*`, `host/ns/*`, shorthands, `!pattern`). **No** Go/RE2 regex override keys.
- Precedence this phase: OVERRIDES > registry (flag lands Phase 2).
- Match key = fully resolved `hostname/namespace/type` after `regaddr.ParseProviderSource` (bare names → `registry.opentofu.org`).
- Split each entry on **first** `=`. Comma-separated entries.
- Deny (`!…`) evaluated **before** positives; first matching positive wins (unless maintainers pick most-specific).
- On mirror match: use mirror protocol; **do not** call disco / `.well-known`.
- Accept `http` and `https` mirror bases (design); normalize trailing `/`.
- Tokens: `TF_TOKEN_*` only — never yaml/params. Encode host: `.` → `_` (Terraform-compatible).
- Do **not** regenerate vendored OT to pull `http_mirror_source.go`; add new hand-written `mirror_source.go` beside vendored files (or carve from generate if needed). Reference upstream: OpenTofu `internal/getproviders/http_mirror_source.go` @ v1.11.4.
- No `--provider-mirror` / `Value.Mirror` in this PR.

---

## File map

| File | Responsibility |
|------|----------------|
| `pkg/vendored/opentofu/getproviders/mirror_source.go` | `MirrorSource` / `NewMirrorSource` — protocol client |
| `pkg/vendored/opentofu/getproviders/mirror_source_test.go` | httptest protocol tests |
| `dynamic/internal/shim/run/mirror_overrides.go` | Parse + match OVERRIDES |
| `dynamic/internal/shim/run/mirror_overrides_test.go` | Grammar / resolve matrix |
| `dynamic/internal/shim/run/tf_token_creds.go` | `TF_TOKEN_*` → `svcauth.CredentialsSource` |
| `dynamic/internal/shim/run/tf_token_creds_test.go` | Host encode + header |
| `dynamic/internal/shim/run/loader.go` | Choose Mirror vs Registry source |
| `dynamic/README.md` | User docs |

---

### Task 1: MirrorSource (protocol client)

**Files:**
- Create: `pkg/vendored/opentofu/getproviders/mirror_source.go`
- Create: `pkg/vendored/opentofu/getproviders/mirror_source_test.go`

**Interfaces:**
- Consumes: `Source`, `addrs.Provider`, `Version`, `Platform`, `PackageMeta`, existing error types (`ErrProviderNotFound`, `ErrUnauthorized`, `ErrQueryFailed`, `ErrPlatformNotSupported`, …)
- Produces:
  ```go
  func NewMirrorSource(baseURL string, creds svcauth.CredentialsSource, httpClient *retryablehttp.Client) (*MirrorSource, error)
  // AvailableVersions / PackageMeta / ForDisplay as Source
  ```

- [ ] **Step 1: Write failing tests** (httptest)

Cover at minimum:

1. `AvailableVersions` → `GET {base}/{host}/{ns}/{type}/index.json` → parse `{"versions":{"1.0.0":{}}}`
2. `PackageMeta` → `GET …/{version}.json` → resolve relative `url` against response URL; absolute URL passthrough
3. Missing provider → 404 → `ErrProviderNotFound` (or bridge-equivalent)
4. 401/403 → unauthorized error mentioning host
5. Platform missing in archives → platform-not-supported
6. Base URL without trailing slash still joins correctly
7. `http://` base accepted (no panic)

Skeleton:

```go
func TestMirrorSource_AvailableVersions(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/providers/registry.terraform.io/hashicorp/random/index.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"versions":{"3.5.1":{}}}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	src, err := NewMirrorSource(srv.URL+"/providers/", nil, nil)
	require.NoError(t, err)

	addr := addrs.Provider{
		Hostname:  svchost.ForComparison("registry.terraform.io"),
		Namespace: "hashicorp",
		Type:      "random",
	}
	vers, _, err := src.AvailableVersions(context.Background(), addr)
	require.NoError(t, err)
	require.True(t, vers.Has(MustParseVersion("3.5.1")))
}
```

Adapt `addrs.Provider` / hostname construction to match this package’s existing tests/helpers.

- [ ] **Step 2: Run tests — expect FAIL** (type missing)

```bash
go test ./pkg/vendored/opentofu/getproviders/ -run TestMirrorSource -count=1
```

- [ ] **Step 3: Implement `MirrorSource`**

Port logic from OpenTofu `HTTPMirrorSource` with these diffs:

- Constructor takes `string` base URL; parse + require `http` or `https`; ensure path ends with `/`.
- Name type `MirrorSource` / `NewMirrorSource` (not `HTTPMirrorSource`).
- Optional `creds`; call `PrepareRequest` when present (same host as mirror base).
- Paths: `path.Join(hostname, namespace, type, "index.json")` and `version+".json"`.
- Do **not** call disco.

- [ ] **Step 4: Run tests — expect PASS**

```bash
go test ./pkg/vendored/opentofu/getproviders/ -run TestMirrorSource -count=1
```

- [ ] **Step 5: Commit**

```bash
git add pkg/vendored/opentofu/getproviders/mirror_source.go pkg/vendored/opentofu/getproviders/mirror_source_test.go
git commit -m "feat(getproviders): add MirrorSource HTTP mirror client"
```

---

### Task 2: OVERRIDES parser + matcher (TF globs)

**Files:**
- Create: `dynamic/internal/shim/run/mirror_overrides.go`
- Create: `dynamic/internal/shim/run/mirror_overrides_test.go`

**Interfaces:**
- Consumes: resolved `addrs.Provider` (or `hostname/ns/type` string)
- Produces:
  ```go
  type MirrorOverrides struct { /* ordered denies + positives */ }

  func ParseMirrorOverrides(s string) (MirrorOverrides, error)
  func (o MirrorOverrides) Resolve(provider addrs.Provider) (mirrorURL string, useMirror bool)
  // useMirror false → caller uses registry
  ```

Env var name: `PULUMI_TF_NETWORK_MIRROR_OVERRIDES`.

Grammar (from design §22.4):

| Key form | Meaning |
|----------|---------|
| `hostname` | All providers on that host |
| `*` | Catch-all |
| `hostname/ns/*`, `hostname/*/*` | Path globs |
| `hashicorp/*` | Shorthand → `registry.terraform.io/hashicorp/*` |
| `*/*` | Shorthand → `registry.terraform.io/*/*` |
| `!pattern` | Deny (same forms); if deny matches → **no mirror** (direct) |

Rules:

1. Split env on commas (trim spaces); empty env → no overrides.
2. Each entry: if starts with `!`, deny (no URL); else split on **first** `=`.
3. Error on empty pattern, empty URL for positives, malformed globs.
4. Reject keys that look like Go regexp (e.g. start with `^` or contain `(?`) — fail parse with clear error.
5. Match order: any deny hit → direct; else first positive match → that URL; else direct.

- [ ] **Step 1: Table tests**

```go
func TestResolveMirrorOverrides(t *testing.T) {
	cases := []struct {
		name, env, addr string
		wantURL         string
		wantMirror      bool
	}{
		{"catch-all", "*=https://m.example/p/", "registry.terraform.io/hashicorp/random", "https://m.example/p/", true},
		{"host", "registry.terraform.io=https://m.example/p/", "registry.terraform.io/hashicorp/random", "https://m.example/p/", true},
		{"glob", "registry.terraform.io/hashicorp/*=https://m.example/p/", "registry.terraform.io/hashicorp/random", "https://m.example/p/", true},
		{"glob miss", "registry.terraform.io/hashicorp/*=https://m.example/p/", "registry.terraform.io/alekc/kubectl", "", false},
		{"shorthand", "hashicorp/*=https://m.example/p/", "registry.terraform.io/hashicorp/random", "https://m.example/p/", true},
		{"deny beats star", "!registry.terraform.io/hashicorp/*,*=https://m.example/p/", "registry.terraform.io/hashicorp/random", "", false},
		{"opentofu bare default host", "registry.opentofu.org/*/*=https://m.example/p/", "hashicorp/random", "https://m.example/p/", true}, // after regaddr resolve
	}
	// Parse addr via regaddr / addrs helpers used by loader; assert Resolve
}
```

Also parse-error cases: `=https://x`, `foo=`, `^registry=https://x`.

- [ ] **Step 2: FAIL then implement `ParseMirrorOverrides` / `Resolve`**

Glob matching: convert TF glob to path segments; `*` matches one segment (not `/`). Exact host key matches when address host equals key and pattern has no `/`.

Shorthand expansion **before** match:

- `hashicorp/*` → `registry.terraform.io/hashicorp/*`
- `*/*` → `registry.terraform.io/*/*`

- [ ] **Step 3: PASS + commit**

```bash
go test ./dynamic/internal/shim/run/ -run 'TestParseMirrorOverrides|TestResolveMirrorOverrides' -count=1
git add dynamic/internal/shim/run/mirror_overrides.go dynamic/internal/shim/run/mirror_overrides_test.go
git commit -m "feat(dynamic): parse PULUMI_TF_NETWORK_MIRROR_OVERRIDES"
```

---

### Task 3: TF_TOKEN_* credentials source

**Files:**
- Create: `dynamic/internal/shim/run/tf_token_creds.go`
- Create: `dynamic/internal/shim/run/tf_token_creds_test.go`

**Interfaces:**
- Produces: `svcauth.CredentialsSource` reading process env
- Host encode: for host `artifactory.example.com` → env `TF_TOKEN_artifactory_example_com`
- Also try IDNA / lowercase forms consistent with Terraform docs

```go
func TFTokenCredentialsFromEnv() svcauth.CredentialsSource
// ForHost → HostCredentialsToken when TF_TOKEN_<encoded> set
```

- [ ] **Step 1: Test encode + PrepareRequest sets Authorization Bearer**

```go
func TestTFTokenCredentials(t *testing.T) {
	t.Setenv("TF_TOKEN_mirror_example_com", "sekrit")
	creds := TFTokenCredentialsFromEnv()
	host, err := svchost.ForComparison("mirror.example.com")
	require.NoError(t, err)
	hc, err := creds.ForHost(context.Background(), host)
	require.NoError(t, err)
	require.NotNil(t, hc)
	req, _ := http.NewRequest("GET", "https://mirror.example.com/x", nil)
	hc.PrepareRequest(req)
	require.Equal(t, "Bearer sekrit", req.Header.Get("Authorization"))
}
```

- [ ] **Step 2: Implement + PASS + commit**

```bash
git commit -m "feat(dynamic): read TF_TOKEN_* for mirror auth"
```

---

### Task 4: Wire loader

**Files:**
- Modify: `dynamic/internal/shim/run/loader.go`
- Modify: `dynamic/internal/shim/run/loader_test.go` (add unit tests; keep integration optional)

**Interfaces:**
- `NamedProvider` stays signature-stable this phase
- Inside `getProviderServer`, after cache miss:

```go
overrides, err := ParseMirrorOverrides(os.Getenv("PULUMI_TF_NETWORK_MIRROR_OVERRIDES"))
// treat parse error as hard fail (return err)
var source getproviders.Source
if url, ok := overrides.Resolve(addr); ok {
    source, err = getproviders.NewMirrorSource(url, TFTokenCredentialsFromEnv(), nil)
} else {
    source = getproviders.NewRegistrySource(ctx, registryDisco, nil, getproviders.LocationConfig{})
}
```

Optional improvement (same PR OK): also attach `TFTokenCredentialsFromEnv()` to `disco.New(...)` so registry private hosts work — only if cheap; do not block mirror path.

- [ ] **Step 1: Unit test with httptest mirror + env**

Set `PULUMI_DYNAMIC_TF_PLUGIN_CACHE_DIR` to temp dir; set OVERRIDES to httptest base; stub index + version JSON + tiny fake zip **or** stop after proving `AvailableVersions` path via injectable source if you refactor a thin seam.

Prefer minimal seam if needed:

```go
// test-only or unexported helper
func selectProviderSource(ctx context.Context, addr addrs.Provider, disco *disco.Disco) (getproviders.Source, error)
```

Test `selectProviderSource` returns `*MirrorSource` vs `*RegistrySource` for env cases — avoids downloading real providers.

- [ ] **Step 2: Implement wire-up**
- [ ] **Step 3: Run**

```bash
go test ./dynamic/internal/shim/run/ -count=1
```

- [ ] **Step 4: Commit**

```bash
git commit -m "feat(dynamic): route provider download via network mirror overrides"
```

---

### Task 5: README docs

**Files:**
- Modify: `dynamic/README.md`

Document:

1. `PULUMI_TF_NETWORK_MIRROR_OVERRIDES` examples (`*`, host, glob, `!`)
2. Bare names resolve to OpenTofu registry — TF-only mirrors need fully qualified `registry.terraform.io/...`
3. `TF_TOKEN_<host>` auth
4. Cache dir interaction (`PULUMI_DYNAMIC_TF_PLUGIN_CACHE_DIR`)
5. Point to design / #3334; note `--provider-mirror` coming Phase 2

- [ ] **Step 1: Edit README**
- [ ] **Step 2: Commit**

```bash
git commit -m "docs(dynamic): document network mirror overrides and TF_TOKEN_*"
```

---

## Phase 1 success criteria

- [ ] `*=URL` / host / TF globs / shorthands / `!` behave per design
- [ ] Mirror path skips disco
- [ ] `TF_TOKEN_*` on mirror metadata requests; 401 actionable
- [ ] Unmatched → existing registry path
- [ ] No `PULUMI_TF_NETWORK_MIRROR_URL`; no RE2 keys; no `--provider-mirror`

## Out of scope

`--provider-mirror`, `Value` persistence, credentials.json, hash verify, filesystem mirror, `.terraformrc`, RE2 keys.
