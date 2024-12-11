# vendored

Bridged providers need to link certain third-party code without depending on it formally via go.mod references.

## Packages

### opentofu

Several internal packages are vendored to access functionality such as:

- `objchange.ProposedNew` implementation
- provider resolution, caching and loading machinery

### terraform-plugin-go

Exposes proto marshaling internals from [terraform-plugin-go](https://github.com/hashicorp/terraform-plugin-go).

### tfplugin6

Borrows TF Protocol 6 proto definitions and rebuilds the Go code for use in bridged providers. This is used to avoid `panic: proto: file "tfplugin6.proto" is already registered` when dependencies pull in separate builds of the same proto definitions.

### tfplugin5

Similarly to `tfplugin6` but builds TF Protocol 5 definitions.

## Notes

The vendored code is currently maintained via `go generate` script and `Makefile`. To re-acquire all the code, run `make vendor`. Consult `generate.go` to edit version references or transformations.

This folder is called `vendored` and not `vendor` to avoid conflicting with Go module vendoring.
