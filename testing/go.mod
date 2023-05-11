module github.com/pulumi/pulumi-terraform-bridge/testing

go 1.19

replace github.com/pulumi/pulumi-terraform-bridge/x/muxer => ../x/muxer

require (
	github.com/stretchr/testify v1.8.2
	google.golang.org/protobuf v1.29.1
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/pulumi/pulumi/sdk/v3 v3.66.1-0.20230504185456-de2b56522344
	golang.org/x/net v0.8.0 // indirect
	golang.org/x/sys v0.6.0 // indirect
	golang.org/x/text v0.8.0 // indirect
	google.golang.org/genproto v0.0.0-20230110181048-76db0878b65f // indirect
	google.golang.org/grpc v1.54.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
