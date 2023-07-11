module github.com/pulumi/pulumi-terraform-bridge/testing

go 1.19

replace github.com/pulumi/pulumi-terraform-bridge/x/muxer => ../x/muxer

require (
	github.com/stretchr/testify v1.8.3
	google.golang.org/protobuf v1.30.0
)

require (
	github.com/kr/text v0.2.0 // indirect
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/pulumi/pulumi/sdk/v3 v3.74.0
	golang.org/x/net v0.8.0 // indirect
	golang.org/x/sys v0.6.0 // indirect
	golang.org/x/text v0.8.0 // indirect
	google.golang.org/genproto v0.0.0-20230306155012-7f2fa6fef1f4 // indirect
	google.golang.org/grpc v1.55.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
