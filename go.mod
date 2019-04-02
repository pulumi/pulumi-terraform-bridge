module github.com/pulumi/pulumi-terraform

go 1.12

require (
	github.com/emirpasic/gods v1.9.0 // indirect
	github.com/gedex/inflector v0.0.0-20170307190818-16278e9db813
	github.com/gliderlabs/ssh v0.1.3 // indirect
	github.com/golang/glog v0.0.0-20160126235308-23def4e6c14b
	github.com/golang/protobuf v1.3.0
	github.com/grpc-ecosystem/grpc-gateway v1.6.2 // indirect
	github.com/hashicorp/go-multierror v1.0.0
	github.com/hashicorp/hcl v0.0.0-20170504190234-a4b07c25de5f
	github.com/hashicorp/terraform v0.12.0-alpha4.0.20190401213546-16778fea9219
	github.com/kevinburke/ssh_config v0.0.0-20180317175531-9fc7bb800b55 // indirect
	github.com/mitchellh/copystructure v1.0.0
	github.com/pkg/errors v0.8.0
	github.com/pulumi/pulumi v0.17.6-0.20190410045519-ef5e148a73c0
	github.com/spf13/cobra v0.0.0-20171204131325-de2d9c4eca8f
	github.com/stretchr/testify v1.3.0
	golang.org/x/net v0.0.0-20190311183353-d8887717615a
	google.golang.org/grpc v1.19.0
	gopkg.in/src-d/go-billy.v4 v4.2.0 // indirect
	gopkg.in/src-d/go-git.v4 v4.5.0 // indirect
	gopkg.in/warnings.v0 v0.1.2 // indirect
)

replace (
	github.com/Nvveen/Gotty => github.com/ijc25/Gotty v0.0.0-20170406111628-a8b993ba6abd
	github.com/golang/glog => github.com/pulumi/glog v0.0.0-20180820174630-7eaa6ffb71e4
)
