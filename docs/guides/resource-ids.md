# Resource IDs

Every Pulumi resource must have an `"id"` field. `"id"` must be a string, and it must be
set by the provider (`Computed` and *not* `Optional` in Terraform parlance).

## SDKv1 and SDKv2 Based Providers

The ID requirement is easy to satisfy for SDKv{1,2} based providers since both SDKs
require an ID field of type string set on the provider. If the provider being bridged is
based on SDKv1 or SDKv2, then the bridge handles Pulumi's ID field without intervention.

## PF Based Providers

Most PF based providers have an attribute called `"id"` of the right kind for the bridge
to use. If your provider doesn't, then you will see an error when `make tfgen` is
run. Each error has a different kind of resolution.

### ID of the wrong type

```
error: Resource dnsimple_email_forward has a problem: "id" attribute is of type "int", expected type "string". To map this resource consider overriding the SchemaInfo.Type field or specifying ResourceInfo.ComputeID.
```

This error happens when the upstream resource has an ID but it's not a string. If the ID
attribute type is coercible to a string, you can fix it by setting the `SchemaInfo.Type` override
for the `"id"` field:

```go
			"dnsimple_email_forward": {
				Fields: map[string]*tfbridge.SchemaInfo{
					"id": {Type: "string"},
				},
			},
```


For providers[^1] where every resource's ID has the wrong type, you can use a `for` loop to apply this:

```go
	prov.P.ResourcesMap().Range(func(key string, value shim.Resource) bool {
		if value.Schema().Get("id").Type() != shim.TypeString {
			r := prov.Resources[key]
			if r.Fields == nil {
				r.Fields = make(map[string]*tfbridge.SchemaInfo, 1)
			}
			r.Fields["id"] = &tfbridge.SchemaInfo{Type: "string"}
		}
		return true
	})
```

If the type of the `"id"` attribute is not coercible to a string, you must set `ResourceInfo.ComputeID`.


[^1]: https://github.com/pulumi/pulumi-dnsimple/blob/7d7e5f3d88082306f15c3600f3481516ae19454a/provider/resources.go#L126-L140

### ID field is missing

```
error: Resource test_res has a problem: no "id" attribute. To map this resource consider specifying ResourceInfo.ComputeID
```

This error should not occur anymore. For dynamic providers and Plugin Framework resources the bridge falls back to emitting a placeholder
value (`"missing ID"`, exported as `tfbridge.MissingIDPlaceholder`) so schema generation and provider execution no longer fail.
Providing an explicit `ComputeID` keeps the generated provider more aligned with the upstream import story. If you want to delegate the ID field
in Pulumi to another attribute, you should use `tfbridge.DelegateIDField` to produce a `ResourceInfo.ComputeID` compatible function.

When using `ResourceInfo.ComputeID` we typically map it to the "import id" of the resource, but there is nothing that requires it to be set to a
specific value. The resource id and the import id do not need to match. Pulumi requires that each resource has an id (Terraform does not), but
there is not requirement that the id mean anything. This is why we fall back to adding the `"missing ID"` value.

```go
"test_res": {ComputeID: tfbridge.DelegateIDField("id", "testprovider", "https://github.com/pulumi/pulumi-testprovider")}
```

Note that the delegated ID field needs to be a valid property, i.e. if the mapped resource does not have a field called "id",
you may need to map this field to something else:

```go
"test_res": {ComputeID: tfbridge.DelegateIDField("valid_key", "testprovider", "https://github.com/pulumi/pulumi-testprovider")}
```


Otherwise you can pass in any function that complies with:

```go
func(ctx context.Context, state resource.PropertyMap) (resource.ID, error)
```


### ID field is of input type

```
error: Resource test_res has a problem: an "id" input attribute is not allowed. To map this resource specify SchemaInfo.Name and ResourceInfo.ComputeID
```

If the resource has an `"id"` attribute but it is Optional or Required on the TF side, that makes it invalid for use in Pulumi. This can be worked around by renaming the field and specifying the `ResourceInfo.ComputeID` field for the resource:

```go
"test_res": {
	Fields: map[string]*info.Schema{
		"id": {
			Name: "idProperty",
		},
	},
	ComputeID: tfbridge.DelegateIDField(resource.PropertyKey("idProperty"),
		"testprovider", "https://github.com/pulumi/pulumi-testprovider"),
},
```
