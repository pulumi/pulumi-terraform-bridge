# vendored

Bridged providers need to link certain third-party code without depending on it formally via go.mod references.

## packages

### opentofu

Used to access the internal objchange.ProposedNew implementation. This processing happens in the `terraform` CLI
process, but needs to be emulated in the provider process when bridging to Pulumi.

### terraform-plugin-go

Exposes proto marshaling internals from [terraform-plugin-go](https://github.com/hashicorp/terraform-plugin-go).

### tfplugin6

Borrows TF Protocol 6 proto definitions and rebuilds the Go code for use in bridged providers. This is used to avoid
`panic: proto: file "tfplugin6.proto" is already registered` when dependencies pull in separate builds of the same proto
definitions.

## Notes

The vendored code is currently maintained via `go generate` scripts. To re-acquire the code, run `go generate`. Consult
`generate.go` to edit version references or transformations.

This folder is called `vendored` and not `vendor` to avoid conflicting with Go module vendoring.
