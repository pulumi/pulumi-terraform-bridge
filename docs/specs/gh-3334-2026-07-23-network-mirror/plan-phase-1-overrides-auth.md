# Phase 1 — MirrorSource + OVERRIDES + TF_TOKEN_* Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Machine-side Terraform network-mirror routing: download dynamic TF providers via `PULUMI_TF_NETWORK_MIRROR_OVERRIDES` (TF globs + `!deny`) and authenticate with `TF_TOKEN_<host>`.

**Architecture:** Add `MirrorSource` implementing `getproviders.Source` (HTTP mirror protocol, skip disco). Parse OVERRIDES into ordered deny/positive rules; match on regaddr-resolved `hostname/namespace/type`. Wire in `dynamic/internal/shim/run/loader.go` via a **required** `selectProviderSource` seam (anticipates Phase 2 `mirrorURL`). Attach bearer tokens from `TF_TOKEN_*` on mirror metadata HTTP (and same-host archive downloads only).

**Tech Stack:** Go; vendored OpenTofu `getproviders`; `github.com/opentofu/svchost` / `svcauth`; `httptest` for mirror stubs; existing `loader_test.go` patterns.

**Spec:** [design.md](./design.md) §10 Phase 1, §11.4 error hints, §22.4 grammar. **Questions:** [maintainer-questions.md](./maintainer-questions.md) (esp. Q7 — TF globs only).

## Global Constraints

- Pattern language = **Terraform globs only** (exact host, `*`, `host/ns/*`, shorthands, `!pattern`). **No** Go/RE2 regex override keys.
- Precedence this phase: OVERRIDES > registry (flag lands Phase 2).
- Match key = fully resolved `hostname/namespace/type` after `regaddr.ParseProviderSource` (bare names → `registry.opentofu.org`).
- Split each entry on **first** `=`. Comma-separated entries. Document that commas **inside** URLs are unsupported (parse error or split incorrectly — reject ambiguous entries if detectable).
- Deny (`!…`) evaluated **before** positives; first matching positive wins (unless maintainers pick most-specific).
- On mirror match: use mirror protocol; **do not** call disco / `.well-known`.
- Accept `http` and `https` mirror bases (design); reject other schemes; normalize trailing `/`.
- Tokens: `TF_TOKEN_*` only — never yaml/params.
- **`TF_TOKEN_*` host encoding (Terraform-compatible):** lowercase host; `.` → `_`; `-` → `__` (double underscore). Example: `my-host.example.com` → `TF_TOKEN_my__host_example_com`. Port: include if non-default per Terraform docs; add a test.
- On archive download: **do NOT** forward `TF_TOKEN_*` credentials to hosts that differ from the mirror base host (absolute archive URLs may point elsewhere).
- When constructing `PackageMeta` from mirror response, populate `PackageHTTPURL.ClientBuilder` so archive download uses the correct HTTP client (retries; auth headers **only** for same-host archives).
- **REQUIRED testability seam:** extract `selectProviderSource` in Phase 1 with signature that already accepts `mirrorURL string` (empty in Phase 1 callers). Do not leave this optional.
- Do **not** regenerate vendored OT to pull `http_mirror_source.go`; add new hand-written `mirror_source.go` beside vendored files. Reference: OpenTofu `internal/getproviders/http_mirror_source.go` @ v1.11.4.
- No `--provider-mirror` / `Value.Mirror` in this PR.

---

## File map

| File | Responsibility |
|------|----------------|
| `pkg/vendored/opentofu/getproviders/mirror_source.go` | `MirrorSource` / `NewMirrorSource` — protocol client |
| `pkg/vendored/opentofu/getproviders/mirror_source_test.go` | httptest protocol + auth-on-wire + URL resolve tests |
| `dynamic/internal/shim/run/mirror_overrides.go` | Parse + match OVERRIDES |
| `dynamic/internal/shim/run/mirror_overrides_test.go` | Grammar / resolve matrix |
| `dynamic/internal/shim/run/tf_token_creds.go` | `TF_TOKEN_*` → `svcauth.CredentialsSource` |
| `dynamic/internal/shim/run/tf_token_creds_test.go` | Host encode (incl. dash) + header |
| `dynamic/internal/shim/run/loader.go` | `selectProviderSource` + choose Mirror vs Registry |
| `dynamic/internal/shim/run/loader_test.go` | Seam tests (Mirror vs Registry) |
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
  // PackageMeta.Location must be PackageHTTPURL with ClientBuilder set
  ```

- [ ] **Step 1: Write failing tests** (httptest)

Cover at minimum:

1. `TestMirrorSource_AvailableVersions` — `GET {base}/{host}/{ns}/{type}/index.json` → parse `{"versions":{"1.0.0":{}}}`
2. `TestMirrorSource_PackageMeta_RelativeURL` — version.json `"url": "terraform-provider-random_3.5.1_linux_amd64.zip"` (relative); resolved URL under mirror path for that provider
3. `TestMirrorSource_PackageMeta_AbsoluteURL` — absolute `https://cdn.example/...zip` passed through as-is
4. `TestMirrorSource_PackageMeta_ClientBuilder` — with creds, `PackageMeta.Location.(PackageHTTPURL).ClientBuilder` produces client/requests that attach auth for **same-host** archive URL; for **different-host** absolute archive URL, auth must **not** be attached
5. Missing provider → 404 → `ErrProviderNotFound` (or bridge-equivalent)
6. `TestMirrorSource_404ErrorHint` — 404 for provider on `registry.opentofu.org` → error text hints qualifying with `registry.terraform.io/...` (design §11.4)
7. 401/403 → unauthorized error mentioning host
8. Platform missing in archives → platform-not-supported
9. Base URL without trailing slash still joins correctly
10. `http://` base accepted (no panic)
11. `TestMirrorSource_InvalidScheme` — `ftp://...` → construction error
12. `TestMirrorSource_WithCredentials_Integration` — wire creds into `NewMirrorSource`; assert `Authorization: Bearer X` on **index.json** and **version.json** requests
13. `TestMirrorSource_RedirectDoesNotLeakToken` — mirror 302 to different host; auth header **not** sent to redirect target (or document + assert actual policy if library strips differently — prefer no leak)

Skeleton (AvailableVersions):

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

Adapt `addrs.Provider` / hostname helpers to this package.

- [ ] **Step 2: Run tests — expect FAIL**

```bash
go test ./pkg/vendored/opentofu/getproviders/ -run TestMirrorSource -count=1
```

- [ ] **Step 3: Implement `MirrorSource`**

Port logic from OpenTofu `HTTPMirrorSource` with diffs:

- Constructor takes `string` base URL; parse + require `http` or `https`; ensure path ends with `/`.
- Name type `MirrorSource` / `NewMirrorSource`.
- Optional `creds`; `PrepareRequest` on metadata GETs to mirror host.
- Paths: `path.Join(hostname, namespace, type, "index.json")` and `version+".json"`.
- `PackageMeta`: resolve relative URL against response URL; absolute passthrough; set `PackageHTTPURL.ClientBuilder` (auth only when archive host == mirror host).
- Do **not** call disco.
- On 404 for `registry.opentofu.org/...`, wrap/hint about `registry.terraform.io/...`.

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
- Consumes: resolved `addrs.Provider`
- Produces:
  ```go
  type MirrorOverrides struct { /* ordered denies + positives */ }

  func ParseMirrorOverrides(s string) (MirrorOverrides, error)
  func (o MirrorOverrides) Resolve(provider addrs.Provider) (mirrorURL string, useMirror bool)
  ```

Env var: `PULUMI_TF_NETWORK_MIRROR_OVERRIDES`.

Grammar (design §22.4):

| Key form | Meaning |
|----------|---------|
| `hostname` | All providers on that host |
| `*` | Catch-all |
| `hostname/ns/*`, `hostname/*/*` | Path globs |
| `hashicorp/*` | Shorthand → `registry.terraform.io/hashicorp/*` |
| `*/*` | Shorthand → `registry.terraform.io/*/*` |
| `!pattern` | Deny (same forms); deny → **no mirror** |

Rules:

1. Split env on commas (trim spaces); empty / whitespace-only → no overrides.
2. Deny entries start with `!` (no URL); positives split on **first** `=`.
3. Error on empty pattern, empty URL for positives, malformed globs.
4. Reject keys that look like Go regexp (start with `^` or contain `(?`) — clear parse error.
5. Match: any deny hit → direct; else first positive → URL; else direct.
6. Shorthand expansion **before** match: `hashicorp/*` → `registry.terraform.io/hashicorp/*`; `*/*` → `registry.terraform.io/*/*`.
7. Glob: `*` = one path segment (not `/`).

- [ ] **Step 1: Table tests** (`TestResolveMirrorOverrides` / `TestParseMirrorOverrides`)

Required cases:

| name | env | addr (resolved) | wantMirror | notes |
|------|-----|-----------------|------------|-------|
| catch-all | `*=https://m.example/p/` | `registry.terraform.io/hashicorp/random` | true | |
| host | `registry.terraform.io=https://m.example/p/` | `registry.terraform.io/hashicorp/random` | true | |
| glob | `registry.terraform.io/hashicorp/*=…` | `…/hashicorp/random` | true | |
| glob miss | same | `…/alekc/kubectl` | false | |
| shorthand | `hashicorp/*=…` | `registry.terraform.io/hashicorp/random` | true | |
| **shorthand vs opentofu bare** | `hashicorp/*=…` | `registry.opentofu.org/hashicorp/random` | **false** | footgun: bare names ≠ TF shorthand |
| deny beats star | `!registry.terraform.io/hashicorp/*,*=…` | `…/hashicorp/random` | false | |
| deny all | `!*,*=…` | any | false | |
| deny exact host | `!registry.terraform.io,*=…` | TF host | false; other host true | |
| first wins | `host=A,host=B` (same host key twice) | matching | URL `A` | |
| empty env | `""` | any | false | |
| whitespace env | `"  "` | any | false | |
| same-host catch-all | `*=https://art.example/p/` | `art.example/myorg/pkg` | **true** | no auto-skip (D10 dropped) |
| opentofu override | `registry.opentofu.org/*/*=…` | resolved bare `hashicorp/random` | true | |
| comma in URL | `host=https://x.com/a,b/` | — | **parse error** (or document + lock behavior) | commas unsupported in URLs |

Parse errors: `=https://x`, `foo=`, `^registry=https://x`.

- [ ] **Step 2: FAIL then implement**
- [ ] **Step 3: PASS + commit**

```bash
go test ./dynamic/internal/shim/run/ -run 'TestParseMirrorOverrides|TestResolveMirrorOverrides' -count=1
git commit -m "feat(dynamic): parse PULUMI_TF_NETWORK_MIRROR_OVERRIDES"
```

---

### Task 3: TF_TOKEN_* credentials source

**Files:**
- Create: `dynamic/internal/shim/run/tf_token_creds.go`
- Create: `dynamic/internal/shim/run/tf_token_creds_test.go`

**Interfaces:**
```go
func TFTokenCredentialsFromEnv() svcauth.CredentialsSource
// Encoding: lower(host); '-' → '__'; '.' → '_'
```

- [ ] **Step 1: Required tests**

1. `TestTFTokenCredentials` — `TF_TOKEN_mirror_example_com` → Bearer on request
2. `TestTFTokenCredentials_DashEncoding` — host `my-host.example.com` → env `TF_TOKEN_my__host_example_com`
3. `TestTFTokenCredentials_NoToken` — unset → nil / no credentials, no panic
4. `TestTFTokenCredentials_PortInHost` — encode host with non-default port per Terraform rules; assert env lookup

- [ ] **Step 2: Implement + PASS + commit**

```bash
git commit -m "feat(dynamic): read TF_TOKEN_* for mirror auth"
```

---

### Task 4: Wire loader (REQUIRED seam)

**Files:**
- Modify: `dynamic/internal/shim/run/loader.go`
- Modify: `dynamic/internal/shim/run/loader_test.go`

**Interfaces (REQUIRED — anticipate Phase 2):**

```go
// selectProviderSource chooses MirrorSource vs RegistrySource.
// mirrorURL non-empty → always MirrorSource at that URL (Phase 2 flag/Value).
// mirrorURL empty → OVERRIDES resolve, else RegistrySource.
func selectProviderSource(
	ctx context.Context,
	addr addrs.Provider,
	mirrorURL string,
	registryDisco *disco.Disco,
) (getproviders.Source, error)
```

Phase 1 callers pass `mirrorURL == ""`.

Inside `getProviderServer`, after cache miss, call `selectProviderSource` then `AvailableVersions` / `PackageMeta` / `InstallPackage` as today.

```go
source, err := selectProviderSource(ctx, addr, "", registryDisco)
```

`selectProviderSource` body:

```go
if strings.TrimSpace(mirrorURL) != "" {
	return getproviders.NewMirrorSource(mirrorURL, TFTokenCredentialsFromEnv(), nil)
}
overrides, err := ParseMirrorOverrides(os.Getenv("PULUMI_TF_NETWORK_MIRROR_OVERRIDES"))
if err != nil {
	return nil, err // hard fail on malformed env
}
if url, ok := overrides.Resolve(addr); ok {
	return getproviders.NewMirrorSource(url, TFTokenCredentialsFromEnv(), nil)
}
return getproviders.NewRegistrySource(ctx, registryDisco, nil, getproviders.LocationConfig{}), nil
```

Optional (same PR OK, not blocking): attach `TFTokenCredentialsFromEnv()` to `disco.New` for private registry hosts.

- [ ] **Step 1: Required seam tests**

1. `TestSelectProviderSource_MirrorVsRegistry` — OVERRIDES set → `*MirrorSource`; unset → `*RegistrySource`
2. `TestSelectProviderSource_EmptyMirrorURLUsesOverrides` — `mirrorURL=""` + match → Mirror at override URL
3. `TestSelectProviderSource_MalformedOverridesError` — bad env → error (not silent registry)

Prefer type-assert / `ForDisplay` / httptest hit counting — **no** live internet downloads.

Optional stretch (nice, not blocking Phase 1 ship): httptest serving index + version + tiny zip through `InstallPackage` once. Minimum bar remains PackageMeta URL correctness from Task 1.

- [ ] **Step 2: Implement wire-up**
- [ ] **Step 3: Run**

```bash
go test ./dynamic/internal/shim/run/ -count=1
go test ./pkg/vendored/opentofu/getproviders/ -run TestMirrorSource -count=1
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
2. Bare names → OpenTofu registry; TF-only mirrors need `registry.terraform.io/...`
3. Footgun: `hashicorp/*` does **not** match bare `hashicorp/random` (opentofu.org)
4. `TF_TOKEN_*` auth + dash encoding note (`-` → `__`)
5. Cache dir (`PULUMI_DYNAMIC_TF_PLUGIN_CACHE_DIR`)
6. No commas in mirror URLs
7. Point to design / #3334; `--provider-mirror` = Phase 2

- [ ] **Step 1: Edit README**
- [ ] **Step 2: Commit**

```bash
git commit -m "docs(dynamic): document network mirror overrides and TF_TOKEN_*"
```

---

## Phase 1 required test checklist (ship gate)

### MirrorSource
- [ ] `TestMirrorSource_AvailableVersions`
- [ ] `TestMirrorSource_PackageMeta_RelativeURL`
- [ ] `TestMirrorSource_PackageMeta_AbsoluteURL`
- [ ] `TestMirrorSource_PackageMeta_ClientBuilder` (same-host auth / cross-host no auth)
- [ ] `TestMirrorSource_WithCredentials_Integration`
- [ ] `TestMirrorSource_RedirectDoesNotLeakToken`
- [ ] `TestMirrorSource_InvalidScheme`
- [ ] `TestMirrorSource_404ErrorHint`
- [ ] 401/403, platform missing, trailing-slash, http base

### OVERRIDES
- [ ] empty / whitespace env
- [ ] catch-all, host, glob hit/miss, shorthand
- [ ] shorthand does **not** match opentofu-resolved bare name
- [ ] deny beats star; deny all; deny exact host
- [ ] first-wins
- [ ] same-host catch-all still mirrors (no magic skip)
- [ ] RE2-looking keys rejected
- [ ] comma-in-URL behavior locked

### TF_TOKEN
- [ ] bearer header
- [ ] dash → `__` encoding
- [ ] no token → nil
- [ ] port-in-host encoding

### Loader seam
- [ ] `TestSelectProviderSource_MirrorVsRegistry`
- [ ] empty `mirrorURL` uses overrides
- [ ] malformed OVERRIDES → error

## Phase 1 success criteria

- [ ] All checkboxes above green
- [ ] Mirror path skips disco
- [ ] Unmatched → registry
- [ ] No `PULUMI_TF_NETWORK_MIRROR_URL`; no RE2 keys; no `--provider-mirror`

## Out of scope

`--provider-mirror`, `Value` persistence, credentials.json, hash verify, filesystem mirror, `.terraformrc`, RE2 keys.
