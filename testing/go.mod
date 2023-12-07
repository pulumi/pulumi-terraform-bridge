module github.com/pulumi/pulumi-terraform-bridge/testing

go 1.20

replace github.com/pulumi/pulumi-terraform-bridge/x/muxer => ../x/muxer

require (
	github.com/stretchr/testify v1.8.4
	google.golang.org/protobuf v1.31.0
)

require (
	github.com/kr/text v0.2.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20230706204954-ccb25ca9f130 // indirect
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/pulumi/pulumi/sdk/v3 v3.96.0
	golang.org/x/net v0.17.0 // indirect
	golang.org/x/sys v0.13.0 // indirect
	golang.org/x/text v0.13.0 // indirect
	google.golang.org/grpc v1.57.1 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/hashicorp/terraform-plugin-sdk/v2 => github.com/pulumi/terraform-plugin-sdk/v2 v2.0.0-20230912190043-e6d96b3b8f7e
