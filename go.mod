module github.com/pulumi/pulumi-terraform-bridge

go 1.12

replace git.apache.org/thrift.git => github.com/apache/thrift v0.12.0

require (
	github.com/Azure/azure-amqp-common-go v1.1.4 // indirect
	github.com/apache/thrift v0.12.0 // indirect
	github.com/coreos/go-etcd v2.0.0+incompatible // indirect
	github.com/cpuguy83/go-md2man v1.0.10 // indirect
	github.com/gedex/inflector v0.0.0-20170307190818-16278e9db813
	github.com/golang/glog v0.0.0-20160126235308-23def4e6c14b
	github.com/golang/protobuf v1.3.5
	github.com/hashicorp/go-multierror v1.0.0
	github.com/hashicorp/hcl v1.0.0
	github.com/hashicorp/hcl2 v0.0.0-20190821123243-0c888d1241f6
	github.com/hashicorp/terraform-plugin-sdk v1.0.0
	github.com/mitchellh/copystructure v1.0.0
	github.com/openzipkin/zipkin-go v0.1.6 // indirect
	github.com/pkg/errors v0.9.1
	github.com/pulumi/pulumi/pkg v0.0.0-20200322194843-61928f04e052
	github.com/pulumi/pulumi/sdk v0.0.0-20200322194843-61928f04e052
	github.com/reconquest/loreley v0.0.0-20160708080500-2ab6b7470a54 // indirect
	github.com/sabhiram/go-gitignore v0.0.0-20180611051255-d3107576ba94 // indirect
	github.com/spf13/cobra v0.0.6
	github.com/stretchr/testify v1.5.1
	github.com/tidwall/pretty v0.0.0-20190325153808-1166b9ac2b65 // indirect
	github.com/uber-go/atomic v1.3.2 // indirect
	github.com/ugorji/go/codec v0.0.0-20181204163529-d75b2dcb6bc8 // indirect
	github.com/xdg/scram v0.0.0-20180814205039-7eeb5667e42c // indirect
	github.com/xdg/stringprep v1.0.0 // indirect
	go.mongodb.org/mongo-driver v1.0.1 // indirect
	golang.org/x/mod v0.2.0
	golang.org/x/net v0.0.0-20200301022130-244492dfa37a
	google.golang.org/grpc v1.28.0
)

replace github.com/Azure/go-autorest => github.com/Azure/go-autorest v12.4.3+incompatible
