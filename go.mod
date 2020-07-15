module github.com/pulumi/pulumi-terraform-bridge/v2

go 1.14

require (
	github.com/blang/semver v3.5.1+incompatible
	github.com/gedex/inflector v0.0.0-20170307190818-16278e9db813
	github.com/golang/glog v0.0.0-20160126235308-23def4e6c14b
	github.com/golang/protobuf v1.3.5
	github.com/hashicorp/go-multierror v1.0.0
	github.com/hashicorp/hcl v1.0.0
	github.com/hashicorp/hcl/v2 v2.3.0
	github.com/hashicorp/terraform-plugin-sdk v1.7.0
	github.com/mitchellh/copystructure v1.0.0
	github.com/pkg/errors v0.9.1
	github.com/pulumi/pulumi/pkg/v2 v2.6.2-0.20200715231137-1a10e06588f6
	github.com/pulumi/pulumi/sdk/v2 v2.6.2-0.20200715231137-1a10e06588f6
	github.com/pulumi/tf2pulumi v0.8.1-0.20200701212620-1e7538fb9a34
	github.com/spf13/afero v1.2.2
	github.com/spf13/cobra v1.0.0
	github.com/stretchr/testify v1.6.1
	golang.org/x/mod v0.2.0
	golang.org/x/net v0.0.0-20200301022130-244492dfa37a
	google.golang.org/grpc v1.28.0
)

replace github.com/Azure/go-autorest => github.com/Azure/go-autorest v12.4.3+incompatible
