module github.com/pulumi/pulumi-terraform-bridge/v2

go 1.14

require (
	github.com/blang/semver v3.5.1+incompatible
	github.com/gedex/inflector v0.0.0-20170307190818-16278e9db813
	github.com/golang/glog v0.0.0-20160126235308-23def4e6c14b
	github.com/golang/protobuf v1.4.2
	github.com/hashicorp/go-multierror v1.0.0
	github.com/hashicorp/hcl v1.0.0
	github.com/hashicorp/hcl/v2 v2.3.0
	github.com/hashicorp/terraform-plugin-sdk v1.7.0
	github.com/mitchellh/copystructure v1.0.0
	github.com/pkg/errors v0.9.1
	github.com/pulumi/pulumi/pkg/v2 v2.9.2-0.20200828155502-7eb99fe7bb15
	github.com/pulumi/pulumi/sdk/v2 v2.9.2-0.20200828155502-7eb99fe7bb15
	github.com/pulumi/tf2pulumi v0.8.1-0.20200701212620-1e7538fb9a34
	github.com/spf13/afero v1.2.2
	github.com/spf13/cobra v1.0.0
	github.com/stretchr/testify v1.6.1
	golang.org/x/mod v0.3.0
	golang.org/x/net v0.0.0-20200602114024-627f9648deb9
	google.golang.org/grpc v1.29.1
)

replace github.com/Azure/go-autorest => github.com/Azure/go-autorest v12.4.3+incompatible
