module github.com/pulumi/pulumi-terraform-bridge

go 1.12

replace git.apache.org/thrift.git => github.com/apache/thrift v0.12.0

require (
	cloud.google.com/go/logging v1.0.0 // indirect
	github.com/blang/semver v3.5.1+incompatible
	github.com/gedex/inflector v0.0.0-20170307190818-16278e9db813
	github.com/golang/glog v0.0.0-20160126235308-23def4e6c14b
	github.com/golang/protobuf v1.3.2
	github.com/hashicorp/go-multierror v1.0.0
	github.com/hashicorp/hcl v1.0.0
	github.com/hashicorp/terraform-plugin-sdk v1.0.0
	github.com/mitchellh/copystructure v1.0.0
	github.com/pkg/errors v0.8.1
	github.com/pulumi/pulumi v1.1.0
	github.com/spf13/cobra v0.0.5
	github.com/stretchr/testify v1.4.0
	golang.org/x/net v0.0.0-20190926025831-c00fd9afed17
	google.golang.org/grpc v1.24.0
)
