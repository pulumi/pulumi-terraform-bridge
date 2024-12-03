// Deprecated: use the github.com/pulumi/pulumi-terraform-bridge/v3 module instead.

module github.com/pulumi/pulumi-terraform-bridge/testing

go 1.22.0

toolchain go1.23.1

replace github.com/pulumi/pulumi-terraform-bridge/v3 => ../

require github.com/pulumi/pulumi-terraform-bridge/v3 v3.0.0-00010101000000-000000000000

require (
	github.com/kr/text v0.2.0 // indirect
	github.com/stretchr/testify v1.9.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240814211410-ddb44dafa142 // indirect
	google.golang.org/protobuf v1.35.1 // indirect
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/pulumi/pulumi/sdk/v3 v3.142.0 // indirect
	golang.org/x/net v0.28.0 // indirect
	golang.org/x/sys v0.24.0 // indirect
	golang.org/x/text v0.17.0 // indirect
	google.golang.org/grpc v1.67.1 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/hashicorp/terraform-plugin-sdk/v2 => github.com/pulumi/terraform-plugin-sdk/v2 v2.0.0-20240520223432-0c0bf0d65f10
