module github.com/pulumi/pulumi-terraform-bridge/testing

go 1.20

replace github.com/pulumi/pulumi-terraform-bridge/x/muxer => ../x/muxer

require (
	github.com/stretchr/testify v1.9.0
	google.golang.org/protobuf v1.33.0
)

require (
	github.com/kr/text v0.2.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240227224415-6ceb2ff114de // indirect
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/pulumi/pulumi/sdk/v3 v3.118.0
	golang.org/x/net v0.26.0 // indirect
	golang.org/x/sys v0.21.0 // indirect
	golang.org/x/text v0.16.0 // indirect
	google.golang.org/grpc v1.63.2 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/hashicorp/terraform-plugin-sdk/v2 => github.com/pulumi/terraform-plugin-sdk/v2 v2.0.0-20240520223432-0c0bf0d65f10
