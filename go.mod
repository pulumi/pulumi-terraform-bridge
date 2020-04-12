module github.com/pulumi/pulumi-terraform-bridge/v2

go 1.13

require (
	github.com/gedex/inflector v0.0.0-20170307190818-16278e9db813
	github.com/golang/glog v0.0.0-20160126235308-23def4e6c14b
	github.com/golang/protobuf v1.3.5
	github.com/hashicorp/go-multierror v1.0.0
	github.com/hashicorp/hcl v1.0.0
	github.com/hashicorp/hcl2 v0.0.0-20190821123243-0c888d1241f6
	github.com/hashicorp/terraform-plugin-sdk v1.0.0
	github.com/mitchellh/copystructure v1.0.0
	github.com/pkg/errors v0.9.1
	github.com/pulumi/pulumi/pkg/v2 v2.0.0-beta.3.0.20200412211925-02a8746a9bcc
	github.com/pulumi/pulumi/sdk/v2 v2.0.0-beta.3.0.20200412211925-02a8746a9bcc
	github.com/spf13/cobra v0.0.6
	github.com/stretchr/testify v1.5.1
	golang.org/x/mod v0.2.0
	golang.org/x/net v0.0.0-20200301022130-244492dfa37a
	google.golang.org/grpc v1.28.0
)

replace github.com/Azure/go-autorest => github.com/Azure/go-autorest v12.4.3+incompatible
