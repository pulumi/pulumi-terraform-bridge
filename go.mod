module github.com/pulumi/pulumi-terraform-bridge

go 1.12

replace git.apache.org/thrift.git => github.com/apache/thrift v0.12.0

require (
	cloud.google.com/go/logging v1.0.0 // indirect
	github.com/Azure/azure-amqp-common-go v1.1.4 // indirect
	github.com/apache/thrift v0.12.0 // indirect
	github.com/gedex/inflector v0.0.0-20170307190818-16278e9db813
	github.com/golang/glog v0.0.0-20160126235308-23def4e6c14b
	github.com/golang/protobuf v1.3.2
	github.com/hashicorp/go-multierror v1.0.0
	github.com/hashicorp/hcl v1.0.0
	github.com/hashicorp/hcl2 v0.0.0-20190821123243-0c888d1241f6
	github.com/hashicorp/terraform-plugin-sdk v1.0.0
	github.com/mitchellh/copystructure v1.0.0
	github.com/openzipkin/zipkin-go v0.1.6 // indirect
	github.com/pkg/errors v0.8.1
	github.com/prometheus/client_golang v0.9.3-0.20190127221311-3c4408c8b829 // indirect
	github.com/pulumi/pulumi v1.11.2-0.20200227180434-db559214e8e9
	github.com/reconquest/loreley v0.0.0-20160708080500-2ab6b7470a54 // indirect
	github.com/sabhiram/go-gitignore v0.0.0-20180611051255-d3107576ba94 // indirect
	github.com/spf13/cobra v0.0.5
	github.com/stretchr/testify v1.4.1-0.20191106224347-f1bd0923b832
	github.com/tidwall/pretty v0.0.0-20190325153808-1166b9ac2b65 // indirect
	github.com/uber-go/atomic v1.3.2 // indirect
	github.com/xdg/scram v0.0.0-20180814205039-7eeb5667e42c // indirect
	github.com/xdg/stringprep v1.0.0 // indirect
	go.mongodb.org/mongo-driver v1.0.1 // indirect
	go.uber.org/atomic v1.3.2 // indirect
	golang.org/x/mod v0.2.0
	golang.org/x/net v0.0.0-20190926025831-c00fd9afed17
	google.golang.org/grpc v1.24.0
)

replace github.com/Azure/go-autorest => github.com/Azure/go-autorest v12.4.3+incompatible
