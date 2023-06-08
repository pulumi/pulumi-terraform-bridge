module github.com/pulumi/pulumi-terraform-bridge/testing

go 1.19

replace github.com/pulumi/pulumi-terraform-bridge/x/muxer => ../x/muxer

require (
	github.com/stretchr/testify v1.8.2
	google.golang.org/protobuf v1.28.1
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/pulumi/pulumi/sdk/v3 v3.69.0
	golang.org/x/net v0.7.0 // indirect
	golang.org/x/sys v0.5.0 // indirect
	golang.org/x/text v0.7.0 // indirect
	google.golang.org/genproto v0.0.0-20220802133213-ce4fa296bf78 // indirect
	google.golang.org/grpc v1.51.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
