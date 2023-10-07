# tfbridge

## Automatic Aliasing (`ApplyAutoAliases`)

Automatically applies backwards compatibility best practices.

The goal is to prevent breaking changes from Pulumi maintainers or from the upstream
provider from causing breaking changes in minor version bumps. We do this by deferring
certain types of breaking changes to major versions.

ApplyAutoAliases attempts to mitigate 3 types of unwanted breaking changes:

- The token mapping for a resource has changed. For example:

	The maintainer is correcting a typo in a manual resource mapping.

- The token mapping for a resource has changed, and a major update caused us to remove
the old name from the schema.

- The upstream provider has added or removed `MaxItems: 1` from a field.

[ApplyAutoAliases] applies three mitigation strategies: one for each breaking change it
is attempting to mitigate.

- Call [ProviderInfo.RenameResourceWithAlias] or [ProviderInfo.RenameDataSource]: This
creates a "hard alias", preventing the user from experiencing a breaking change between
major versions.

- Edit [ResourceInfo.Aliases]: This creates a "soft alias", making it easier for users
to move to the new resource when the old resource is removed.

- Edit [SchemaInfo.MaxItemsOne]: This allows us to defer MaxItemsOne changes to the
next major release.

All mitigations act on [ProviderInfo.Resources] / [ProviderInfo.DataSources]. These
mitigations are then propagated to the schema (if during tfgen) or used at runtime (at
runtime). Conceptually, [ApplyAutoAliases] performs the same kind of mitigations that a
careful provider author would perform manually: invoking
[ProviderInfo.RenameResourceWithAlias], adding token aliases for resources that have
been moved, and fixing MaxItemsOne to avoid backwards compatibility breaks.

The goal is to always maximize backwards compatibility and reduce breaking changes for
the users of the Pulumi providers. The basic functionality behind each action is
identical; ApplyAutoAliases keeps a record of which TF token maps to which Pulumi
token, which fields have MaxItemsOne (true or false), and what version the record is
from.

The dataflow for aliases history goes like this:

``` mermaid
flowchart TD
    A["Field History\n(bridge-metadata.json)"] -->|go:embed| B["resources.go: func Provider()"]
    B --> C["Token Mapping\n(manual & automatic)"]
    C --> D["Manual aliasing/MaxItemsOne application"]
    D -->|"make tfgen or runtime"| E["prov.ApplyAutoAliasing*\nfrom pulumi-terraform-bridge/pkg\n\nIterates through all resources and datasources,\nto applying and recording:\n1. Hard aliases**\n2. Soft aliases***\n3. MaxItemsOne corrections"]
    E -->|"Iterate through resources and datasources to apply"| F["New aliasing history"] 
    E -->|"Always (ignored during tfgen time)"| G["Runtime aliasing information"]
    F -->|"save history (make tfgen ONLY)"| A
```

> (\*)   This gets called anytime Provider() gets called.
>
> (\*\*) A "hard alias" is a fully schematized copy of a resource, where we
> ensure. backwards compatibility for new token names. Mainly used during minor upgrades.
>
> (\*\*\*) A "soft alias" is a plain rename of a token, which requires a user to rename
> the resource (but not do an stack surgery). Only a resource rename is needed. Acceptable
> as a result of major upgrades

This records is stored using the
github.com/pulumi/pulumi-terraform-bridge/unstable/metadata interface. It is written to
at tfgen time and read from when starting up a provider (tfgen time and normal runtime).

For example, this is the (abbreviated & modified) history for GCP's compute autoscalar:

```
	"google_compute_autoscaler": {
	    "current": "gcp:compute/autoscalar:Autoscalar",
	    "past": [
	        {
	            "name": "gcp:auto/autoscalar:Autoscalar",
	            "inCodegen": true,
	            "majorVersion": 6
	        },
	        {
	            "name": "gcp:auto/scaler:Scaler",
	            "inCodegen": false,
	            "majorVersion": 5
	        }
	    ],
	    "majorVersion": 6,
	    "fields": {
	        "autoscaling_policy": {
	            "maxItemsOne": true,
	            "elem": {
	                "fields": {
	                    "cpu_utilization": {
	                        "maxItemsOne": false
	                    }
	                }
	            }
	        }
	    }
	}
```

I will address each action as it applies to `"google_compute_autoscaler" in turn:

# Call [ProviderInfo.RenameResourceWithAlias] or [ProviderInfo.RenameDataSource]

"google_compute_autoscaler.majorVersion" tells us that this record was last updated at
major version 6. One of the previous names is also at major version 6, so we want to
keep full backwards compatibility.

ApplyAutoAliases will call [ProviderInfo.RenameResourceWithAlias] to create a SDK entry
for the old Pulumi token
("gcp:auto/autoscalar:Autoscalar"). "gcp:auto/autoscalar:Autoscalar" will no longer be
hard aliased when `make tfgen` is run on version 7, since we are then allowed to make
breaking changes.

# Edit [ResourceInfo.Aliases]

In this history, we have recorded two prior names for "google_compute_autoscaler":
"gcp:auto/autoscalar:Autoscalar" and "gcp:auto/scaler:Scaler". Since
"gcp:auto/scaler:Scaler" was from a previous major version, we don't need to maintain
full backwards compatibility. Instead, we will apply a type alias to
`.Resources["gcp:compute/autoscalar:Autoscalar"].Aliases`: `AliasInfo{Type:
""gcp:auto/scaler:Scaler"}`. This makes it easy for consumers to upgrade from the old
name to the new.

# Edit [SchemaInfo.MaxItemsOne]

The provider has been shipped with fields that could have `MaxItemsOne` applied. Any
change here is breaking to our users, so we prevent it. As long as the provider's major
version is 6, ApplyAutoAliases will override the MaxItemsOne status of
"autoscaling_policy" (to true) and "autoscaling_policy.elem.cpu_utilization" (to
false), regardless of what upstream does. We will read new MaxItemsOne values from the
provider when the next major version (v7 in this example) is released. Effectively this
makes sure that upstream MaxItems changes are deferred until the next major version.

---

Implementation note: to operate correctly this method needs to keep a persistent track
of a database of past decision history. This is currently done by doing reads and
writes to `providerInfo.GetMetadata()`, which is assumed to be persisted across
provider releases. The bridge framework keeps this information written out to an opaque
`bridge-metadata.json` blob which is expected to be stored in source control to persist
across releases.
