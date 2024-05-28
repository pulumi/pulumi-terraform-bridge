Provides a Lambda Function resource. Lambda allows you to trigger execution of code in response to events in AWS, enabling serverless backend solutions. The Lambda Function itself includes source code and runtime configuration.

For information about Lambda and how to use it, see [What is AWS Lambda?][1]

For a detailed example of setting up Lambda and API Gateway, see [Serverless Applications with AWS Lambda and API Gateway.][11]

~> **NOTE:** Due to [AWS Lambda improved VPC networking changes that began deploying in September 2019](https://aws.amazon.com/blogs/compute/announcing-improved-vpc-networking-for-aws-lambda-functions/), EC2 subnets and security groups associated with Lambda Functions can take up to 45 minutes to successfully delete.

~> **NOTE:** If you get a `KMSAccessDeniedException: Lambda was unable to decrypt the environment variables because KMS access was denied` error when invoking an `aws_lambda_function` with environment variables, the IAM role associated with the function may have been deleted and recreated _after_ the function was created. You can fix the problem two ways: 1) updating the function's role to another role and then updating it back again to the recreated role, or 2) by using Terraform to `taint` the function and `apply` your configuration again to recreate the function. (When you create a function, Lambda grants permissions on the KMS key to the function's IAM role. If the IAM role is recreated, the grant is no longer valid. Changing the function's role or recreating the function causes Lambda to update the grant.)

-> To give an external source (like an EventBridge Rule, SNS, or S3) permission to access the Lambda function, use the `aws_lambda_permission` resource. See [Lambda Permission Model][4] for more details. On the other hand, the `role` argument of this resource is the function's execution role for identity and access to AWS services and resources.

## Example Usage

### Basic Example

<!--Start PulumiCodeChooser -->
```typescript
import * as pulumi from "@pulumi/pulumi";
import * as archive from "@pulumi/archive";
import * as aws from "@pulumi/aws";

const assumeRole = aws.iam.getPolicyDocument({
    statements: [{
        effect: "Allow",
        principals: [{
            type: "Service",
            identifiers: ["lambda.amazonaws.com"],
        }],
        actions: ["sts:AssumeRole"],
    }],
});
const iamForLambda = new aws.iam.Role("iam_for_lambda", {
    name: "iam_for_lambda",
    assumeRolePolicy: assumeRole.then(assumeRole => assumeRole.json),
});
const lambda = archive.getFile({
    type: "zip",
    sourceFile: "lambda.js",
    outputPath: "lambda_function_payload.zip",
});
const testLambda = new aws.lambda.Function("test_lambda", {
    code: new pulumi.asset.FileArchive("lambda_function_payload.zip"),
    name: "lambda_function_name",
    role: iamForLambda.arn,
    handler: "index.test",
    sourceCodeHash: lambda.then(lambda => lambda.outputBase64sha256),
    runtime: aws.lambda.Runtime.NodeJS18dX,
    environment: {
        variables: {
            foo: "bar",
        },
    },
});
```
```python
import pulumi
import pulumi_archive as archive
import pulumi_aws as aws

assume_role = aws.iam.get_policy_document(statements=[aws.iam.GetPolicyDocumentStatementArgs(
    effect="Allow",
    principals=[aws.iam.GetPolicyDocumentStatementPrincipalArgs(
        type="Service",
        identifiers=["lambda.amazonaws.com"],
    )],
    actions=["sts:AssumeRole"],
)])
iam_for_lambda = aws.iam.Role("iam_for_lambda",
    name="iam_for_lambda",
    assume_role_policy=assume_role.json)
lambda_ = archive.get_file(type="zip",
    source_file="lambda.js",
    output_path="lambda_function_payload.zip")
test_lambda = aws.lambda_.Function("test_lambda",
    code=pulumi.FileArchive("lambda_function_payload.zip"),
    name="lambda_function_name",
    role=iam_for_lambda.arn,
    handler="index.test",
    source_code_hash=lambda_.output_base64sha256,
    runtime=aws.lambda_.Runtime.NODE_JS18D_X,
    environment=aws.lambda_.FunctionEnvironmentArgs(
        variables={
            "foo": "bar",
        },
    ))
```
```csharp
using System.Collections.Generic;
using System.Linq;
using Pulumi;
using Archive = Pulumi.Archive;
using Aws = Pulumi.Aws;

return await Deployment.RunAsync(() => 
{
    var assumeRole = Aws.Iam.GetPolicyDocument.Invoke(new()
    {
        Statements = new[]
        {
            new Aws.Iam.Inputs.GetPolicyDocumentStatementInputArgs
            {
                Effect = "Allow",
                Principals = new[]
                {
                    new Aws.Iam.Inputs.GetPolicyDocumentStatementPrincipalInputArgs
                    {
                        Type = "Service",
                        Identifiers = new[]
                        {
                            "lambda.amazonaws.com",
                        },
                    },
                },
                Actions = new[]
                {
                    "sts:AssumeRole",
                },
            },
        },
    });

    var iamForLambda = new Aws.Iam.Role("iam_for_lambda", new()
    {
        Name = "iam_for_lambda",
        AssumeRolePolicy = assumeRole.Apply(getPolicyDocumentResult => getPolicyDocumentResult.Json),
    });

    var lambda = Archive.GetFile.Invoke(new()
    {
        Type = "zip",
        SourceFile = "lambda.js",
        OutputPath = "lambda_function_payload.zip",
    });

    var testLambda = new Aws.Lambda.Function("test_lambda", new()
    {
        Code = new FileArchive("lambda_function_payload.zip"),
        Name = "lambda_function_name",
        Role = iamForLambda.Arn,
        Handler = "index.test",
        SourceCodeHash = lambda.Apply(getFileResult => getFileResult.OutputBase64sha256),
        Runtime = Aws.Lambda.Runtime.NodeJS18dX,
        Environment = new Aws.Lambda.Inputs.FunctionEnvironmentArgs
        {
            Variables = 
            {
                { "foo", "bar" },
            },
        },
    });

});
```
```go
package main

import (
	"github.com/pulumi/pulumi-archive/sdk/go/archive"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/iam"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/lambda"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		assumeRole, err := iam.GetPolicyDocument(ctx, &iam.GetPolicyDocumentArgs{
			Statements: []iam.GetPolicyDocumentStatement{
				{
					Effect: pulumi.StringRef("Allow"),
					Principals: []iam.GetPolicyDocumentStatementPrincipal{
						{
							Type: "Service",
							Identifiers: []string{
								"lambda.amazonaws.com",
							},
						},
					},
					Actions: []string{
						"sts:AssumeRole",
					},
				},
			},
		}, nil)
		if err != nil {
			return err
		}
		iamForLambda, err := iam.NewRole(ctx, "iam_for_lambda", &iam.RoleArgs{
			Name:             pulumi.String("iam_for_lambda"),
			AssumeRolePolicy: pulumi.String(assumeRole.Json),
		})
		if err != nil {
			return err
		}
		lambda, err := archive.LookupFile(ctx, &archive.LookupFileArgs{
			Type:       "zip",
			SourceFile: pulumi.StringRef("lambda.js"),
			OutputPath: "lambda_function_payload.zip",
		}, nil)
		if err != nil {
			return err
		}
		_, err = lambda.NewFunction(ctx, "test_lambda", &lambda.FunctionArgs{
			Code:           pulumi.NewFileArchive("lambda_function_payload.zip"),
			Name:           pulumi.String("lambda_function_name"),
			Role:           iamForLambda.Arn,
			Handler:        pulumi.String("index.test"),
			SourceCodeHash: pulumi.String(lambda.OutputBase64sha256),
			Runtime:        pulumi.String(lambda.RuntimeNodeJS18dX),
			Environment: &lambda.FunctionEnvironmentArgs{
				Variables: pulumi.StringMap{
					"foo": pulumi.String("bar"),
				},
			},
		})
		if err != nil {
			return err
		}
		return nil
	})
}
```
```java
package generated_program;

import com.pulumi.Context;
import com.pulumi.Pulumi;
import com.pulumi.core.Output;
import com.pulumi.aws.iam.IamFunctions;
import com.pulumi.aws.iam.inputs.GetPolicyDocumentArgs;
import com.pulumi.aws.iam.Role;
import com.pulumi.aws.iam.RoleArgs;
import com.pulumi.archive.ArchiveFunctions;
import com.pulumi.archive.inputs.GetFileArgs;
import com.pulumi.aws.lambda.Function;
import com.pulumi.aws.lambda.FunctionArgs;
import com.pulumi.aws.lambda.inputs.FunctionEnvironmentArgs;
import com.pulumi.asset.FileArchive;
import java.util.List;
import java.util.ArrayList;
import java.util.Map;
import java.io.File;
import java.nio.file.Files;
import java.nio.file.Paths;

public class App {
    public static void main(String[] args) {
        Pulumi.run(App::stack);
    }

    public static void stack(Context ctx) {
        final var assumeRole = IamFunctions.getPolicyDocument(GetPolicyDocumentArgs.builder()
            .statements(GetPolicyDocumentStatementArgs.builder()
                .effect("Allow")
                .principals(GetPolicyDocumentStatementPrincipalArgs.builder()
                    .type("Service")
                    .identifiers("lambda.amazonaws.com")
                    .build())
                .actions("sts:AssumeRole")
                .build())
            .build());

        var iamForLambda = new Role("iamForLambda", RoleArgs.builder()
            .name("iam_for_lambda")
            .assumeRolePolicy(assumeRole.applyValue(getPolicyDocumentResult -> getPolicyDocumentResult.json()))
            .build());

        final var lambda = ArchiveFunctions.getFile(GetFileArgs.builder()
            .type("zip")
            .sourceFile("lambda.js")
            .outputPath("lambda_function_payload.zip")
            .build());

        var testLambda = new Function("testLambda", FunctionArgs.builder()
            .code(new FileArchive("lambda_function_payload.zip"))
            .name("lambda_function_name")
            .role(iamForLambda.arn())
            .handler("index.test")
            .sourceCodeHash(lambda.applyValue(getFileResult -> getFileResult.outputBase64sha256()))
            .runtime("nodejs18.x")
            .environment(FunctionEnvironmentArgs.builder()
                .variables(Map.of("foo", "bar"))
                .build())
            .build());

    }
}
```
```yaml
resources:
  iamForLambda:
    type: aws:iam:Role
    name: iam_for_lambda
    properties:
      name: iam_for_lambda
      assumeRolePolicy: ${assumeRole.json}
  testLambda:
    type: aws:lambda:Function
    name: test_lambda
    properties:
      code:
        fn::FileArchive: lambda_function_payload.zip
      name: lambda_function_name
      role: ${iamForLambda.arn}
      handler: index.test
      sourceCodeHash: ${lambda.outputBase64sha256}
      runtime: nodejs18.x
      environment:
        variables:
          foo: bar
variables:
  assumeRole:
    fn::invoke:
      Function: aws:iam:getPolicyDocument
      Arguments:
        statements:
          - effect: Allow
            principals:
              - type: Service
                identifiers:
                  - lambda.amazonaws.com
            actions:
              - sts:AssumeRole
  lambda:
    fn::invoke:
      Function: archive:getFile
      Arguments:
        type: zip
        sourceFile: lambda.js
        outputPath: lambda_function_payload.zip
```
<!--End PulumiCodeChooser -->

### Lambda Layers

<!--Start PulumiCodeChooser -->
```typescript
import * as pulumi from "@pulumi/pulumi";
import * as aws from "@pulumi/aws";

const example = new aws.lambda.LayerVersion("example", {});
const exampleFunction = new aws.lambda.Function("example", {layers: [example.arn]});
```
```python
import pulumi
import pulumi_aws as aws

example = aws.lambda_.LayerVersion("example")
example_function = aws.lambda_.Function("example", layers=[example.arn])
```
```csharp
using System.Collections.Generic;
using System.Linq;
using Pulumi;
using Aws = Pulumi.Aws;

return await Deployment.RunAsync(() => 
{
    var example = new Aws.Lambda.LayerVersion("example");

    var exampleFunction = new Aws.Lambda.Function("example", new()
    {
        Layers = new[]
        {
            example.Arn,
        },
    });

});
```
```go
package main

import (
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/lambda"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		example, err := lambda.NewLayerVersion(ctx, "example", nil)
		if err != nil {
			return err
		}
		_, err = lambda.NewFunction(ctx, "example", &lambda.FunctionArgs{
			Layers: pulumi.StringArray{
				example.Arn,
			},
		})
		if err != nil {
			return err
		}
		return nil
	})
}
```
```java
package generated_program;

import com.pulumi.Context;
import com.pulumi.Pulumi;
import com.pulumi.core.Output;
import com.pulumi.aws.lambda.LayerVersion;
import com.pulumi.aws.lambda.Function;
import com.pulumi.aws.lambda.FunctionArgs;
import java.util.List;
import java.util.ArrayList;
import java.util.Map;
import java.io.File;
import java.nio.file.Files;
import java.nio.file.Paths;

public class App {
    public static void main(String[] args) {
        Pulumi.run(App::stack);
    }

    public static void stack(Context ctx) {
        var example = new LayerVersion("example");

        var exampleFunction = new Function("exampleFunction", FunctionArgs.builder()
            .layers(example.arn())
            .build());

    }
}
```
```yaml
resources:
  example:
    type: aws:lambda:LayerVersion
  exampleFunction:
    type: aws:lambda:Function
    name: example
    properties:
      layers:
        - ${example.arn}
```
<!--End PulumiCodeChooser -->

### Lambda Ephemeral Storage

Lambda Function Ephemeral Storage(`/tmp`) allows you to configure the storage upto `10` GB. The default value set to `512` MB.

<!--Start PulumiCodeChooser -->
```typescript
import * as pulumi from "@pulumi/pulumi";
import * as aws from "@pulumi/aws";

const assumeRole = aws.iam.getPolicyDocument({
    statements: [{
        effect: "Allow",
        principals: [{
            type: "Service",
            identifiers: ["lambda.amazonaws.com"],
        }],
        actions: ["sts:AssumeRole"],
    }],
});
const iamForLambda = new aws.iam.Role("iam_for_lambda", {
    name: "iam_for_lambda",
    assumeRolePolicy: assumeRole.then(assumeRole => assumeRole.json),
});
const testLambda = new aws.lambda.Function("test_lambda", {
    code: new pulumi.asset.FileArchive("lambda_function_payload.zip"),
    name: "lambda_function_name",
    role: iamForLambda.arn,
    handler: "index.test",
    runtime: aws.lambda.Runtime.NodeJS18dX,
    ephemeralStorage: {
        size: 10240,
    },
});
```
```python
import pulumi
import pulumi_aws as aws

assume_role = aws.iam.get_policy_document(statements=[aws.iam.GetPolicyDocumentStatementArgs(
    effect="Allow",
    principals=[aws.iam.GetPolicyDocumentStatementPrincipalArgs(
        type="Service",
        identifiers=["lambda.amazonaws.com"],
    )],
    actions=["sts:AssumeRole"],
)])
iam_for_lambda = aws.iam.Role("iam_for_lambda",
    name="iam_for_lambda",
    assume_role_policy=assume_role.json)
test_lambda = aws.lambda_.Function("test_lambda",
    code=pulumi.FileArchive("lambda_function_payload.zip"),
    name="lambda_function_name",
    role=iam_for_lambda.arn,
    handler="index.test",
    runtime=aws.lambda_.Runtime.NODE_JS18D_X,
    ephemeral_storage=aws.lambda_.FunctionEphemeralStorageArgs(
        size=10240,
    ))
```
```csharp
using System.Collections.Generic;
using System.Linq;
using Pulumi;
using Aws = Pulumi.Aws;

return await Deployment.RunAsync(() => 
{
    var assumeRole = Aws.Iam.GetPolicyDocument.Invoke(new()
    {
        Statements = new[]
        {
            new Aws.Iam.Inputs.GetPolicyDocumentStatementInputArgs
            {
                Effect = "Allow",
                Principals = new[]
                {
                    new Aws.Iam.Inputs.GetPolicyDocumentStatementPrincipalInputArgs
                    {
                        Type = "Service",
                        Identifiers = new[]
                        {
                            "lambda.amazonaws.com",
                        },
                    },
                },
                Actions = new[]
                {
                    "sts:AssumeRole",
                },
            },
        },
    });

    var iamForLambda = new Aws.Iam.Role("iam_for_lambda", new()
    {
        Name = "iam_for_lambda",
        AssumeRolePolicy = assumeRole.Apply(getPolicyDocumentResult => getPolicyDocumentResult.Json),
    });

    var testLambda = new Aws.Lambda.Function("test_lambda", new()
    {
        Code = new FileArchive("lambda_function_payload.zip"),
        Name = "lambda_function_name",
        Role = iamForLambda.Arn,
        Handler = "index.test",
        Runtime = Aws.Lambda.Runtime.NodeJS18dX,
        EphemeralStorage = new Aws.Lambda.Inputs.FunctionEphemeralStorageArgs
        {
            Size = 10240,
        },
    });

});
```
```go
package main

import (
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/iam"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/lambda"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		assumeRole, err := iam.GetPolicyDocument(ctx, &iam.GetPolicyDocumentArgs{
			Statements: []iam.GetPolicyDocumentStatement{
				{
					Effect: pulumi.StringRef("Allow"),
					Principals: []iam.GetPolicyDocumentStatementPrincipal{
						{
							Type: "Service",
							Identifiers: []string{
								"lambda.amazonaws.com",
							},
						},
					},
					Actions: []string{
						"sts:AssumeRole",
					},
				},
			},
		}, nil)
		if err != nil {
			return err
		}
		iamForLambda, err := iam.NewRole(ctx, "iam_for_lambda", &iam.RoleArgs{
			Name:             pulumi.String("iam_for_lambda"),
			AssumeRolePolicy: pulumi.String(assumeRole.Json),
		})
		if err != nil {
			return err
		}
		_, err = lambda.NewFunction(ctx, "test_lambda", &lambda.FunctionArgs{
			Code:    pulumi.NewFileArchive("lambda_function_payload.zip"),
			Name:    pulumi.String("lambda_function_name"),
			Role:    iamForLambda.Arn,
			Handler: pulumi.String("index.test"),
			Runtime: pulumi.String(lambda.RuntimeNodeJS18dX),
			EphemeralStorage: &lambda.FunctionEphemeralStorageArgs{
				Size: pulumi.Int(10240),
			},
		})
		if err != nil {
			return err
		}
		return nil
	})
}
```
```java
package generated_program;

import com.pulumi.Context;
import com.pulumi.Pulumi;
import com.pulumi.core.Output;
import com.pulumi.aws.iam.IamFunctions;
import com.pulumi.aws.iam.inputs.GetPolicyDocumentArgs;
import com.pulumi.aws.iam.Role;
import com.pulumi.aws.iam.RoleArgs;
import com.pulumi.aws.lambda.Function;
import com.pulumi.aws.lambda.FunctionArgs;
import com.pulumi.aws.lambda.inputs.FunctionEphemeralStorageArgs;
import com.pulumi.asset.FileArchive;
import java.util.List;
import java.util.ArrayList;
import java.util.Map;
import java.io.File;
import java.nio.file.Files;
import java.nio.file.Paths;

public class App {
    public static void main(String[] args) {
        Pulumi.run(App::stack);
    }

    public static void stack(Context ctx) {
        final var assumeRole = IamFunctions.getPolicyDocument(GetPolicyDocumentArgs.builder()
            .statements(GetPolicyDocumentStatementArgs.builder()
                .effect("Allow")
                .principals(GetPolicyDocumentStatementPrincipalArgs.builder()
                    .type("Service")
                    .identifiers("lambda.amazonaws.com")
                    .build())
                .actions("sts:AssumeRole")
                .build())
            .build());

        var iamForLambda = new Role("iamForLambda", RoleArgs.builder()
            .name("iam_for_lambda")
            .assumeRolePolicy(assumeRole.applyValue(getPolicyDocumentResult -> getPolicyDocumentResult.json()))
            .build());

        var testLambda = new Function("testLambda", FunctionArgs.builder()
            .code(new FileArchive("lambda_function_payload.zip"))
            .name("lambda_function_name")
            .role(iamForLambda.arn())
            .handler("index.test")
            .runtime("nodejs18.x")
            .ephemeralStorage(FunctionEphemeralStorageArgs.builder()
                .size(10240)
                .build())
            .build());

    }
}
```
```yaml
resources:
  iamForLambda:
    type: aws:iam:Role
    name: iam_for_lambda
    properties:
      name: iam_for_lambda
      assumeRolePolicy: ${assumeRole.json}
  testLambda:
    type: aws:lambda:Function
    name: test_lambda
    properties:
      code:
        fn::FileArchive: lambda_function_payload.zip
      name: lambda_function_name
      role: ${iamForLambda.arn}
      handler: index.test
      runtime: nodejs18.x
      ephemeralStorage:
        size: 10240
variables:
  assumeRole:
    fn::invoke:
      Function: aws:iam:getPolicyDocument
      Arguments:
        statements:
          - effect: Allow
            principals:
              - type: Service
                identifiers:
                  - lambda.amazonaws.com
            actions:
              - sts:AssumeRole
```
<!--End PulumiCodeChooser -->

### Lambda File Systems

Lambda File Systems allow you to connect an Amazon Elastic File System (EFS) file system to a Lambda function to share data across function invocations, access existing data including large files, and save function state.

<!--Start PulumiCodeChooser -->
```typescript
import * as pulumi from "@pulumi/pulumi";
import * as aws from "@pulumi/aws";

// EFS file system
const efsForLambda = new aws.efs.FileSystem("efs_for_lambda", {tags: {
    Name: "efs_for_lambda",
}});
// Mount target connects the file system to the subnet
const alpha = new aws.efs.MountTarget("alpha", {
    fileSystemId: efsForLambda.id,
    subnetId: subnetForLambda.id,
    securityGroups: [sgForLambda.id],
});
// EFS access point used by lambda file system
const accessPointForLambda = new aws.efs.AccessPoint("access_point_for_lambda", {
    fileSystemId: efsForLambda.id,
    rootDirectory: {
        path: "/lambda",
        creationInfo: {
            ownerGid: 1000,
            ownerUid: 1000,
            permissions: "777",
        },
    },
    posixUser: {
        gid: 1000,
        uid: 1000,
    },
});
// A lambda function connected to an EFS file system
const example = new aws.lambda.Function("example", {
    fileSystemConfig: {
        arn: accessPointForLambda.arn,
        localMountPath: "/mnt/efs",
    },
    vpcConfig: {
        subnetIds: [subnetForLambda.id],
        securityGroupIds: [sgForLambda.id],
    },
}, {
    dependsOn: [alpha],
});
```
```python
import pulumi
import pulumi_aws as aws

# EFS file system
efs_for_lambda = aws.efs.FileSystem("efs_for_lambda", tags={
    "Name": "efs_for_lambda",
})
# Mount target connects the file system to the subnet
alpha = aws.efs.MountTarget("alpha",
    file_system_id=efs_for_lambda.id,
    subnet_id=subnet_for_lambda["id"],
    security_groups=[sg_for_lambda["id"]])
# EFS access point used by lambda file system
access_point_for_lambda = aws.efs.AccessPoint("access_point_for_lambda",
    file_system_id=efs_for_lambda.id,
    root_directory=aws.efs.AccessPointRootDirectoryArgs(
        path="/lambda",
        creation_info=aws.efs.AccessPointRootDirectoryCreationInfoArgs(
            owner_gid=1000,
            owner_uid=1000,
            permissions="777",
        ),
    ),
    posix_user=aws.efs.AccessPointPosixUserArgs(
        gid=1000,
        uid=1000,
    ))
# A lambda function connected to an EFS file system
example = aws.lambda_.Function("example",
    file_system_config=aws.lambda_.FunctionFileSystemConfigArgs(
        arn=access_point_for_lambda.arn,
        local_mount_path="/mnt/efs",
    ),
    vpc_config=aws.lambda_.FunctionVpcConfigArgs(
        subnet_ids=[subnet_for_lambda["id"]],
        security_group_ids=[sg_for_lambda["id"]],
    ),
    opts=pulumi.ResourceOptions(depends_on=[alpha]))
```
```csharp
using System.Collections.Generic;
using System.Linq;
using Pulumi;
using Aws = Pulumi.Aws;

return await Deployment.RunAsync(() => 
{
    // EFS file system
    var efsForLambda = new Aws.Efs.FileSystem("efs_for_lambda", new()
    {
        Tags = 
        {
            { "Name", "efs_for_lambda" },
        },
    });

    // Mount target connects the file system to the subnet
    var alpha = new Aws.Efs.MountTarget("alpha", new()
    {
        FileSystemId = efsForLambda.Id,
        SubnetId = subnetForLambda.Id,
        SecurityGroups = new[]
        {
            sgForLambda.Id,
        },
    });

    // EFS access point used by lambda file system
    var accessPointForLambda = new Aws.Efs.AccessPoint("access_point_for_lambda", new()
    {
        FileSystemId = efsForLambda.Id,
        RootDirectory = new Aws.Efs.Inputs.AccessPointRootDirectoryArgs
        {
            Path = "/lambda",
            CreationInfo = new Aws.Efs.Inputs.AccessPointRootDirectoryCreationInfoArgs
            {
                OwnerGid = 1000,
                OwnerUid = 1000,
                Permissions = "777",
            },
        },
        PosixUser = new Aws.Efs.Inputs.AccessPointPosixUserArgs
        {
            Gid = 1000,
            Uid = 1000,
        },
    });

    // A lambda function connected to an EFS file system
    var example = new Aws.Lambda.Function("example", new()
    {
        FileSystemConfig = new Aws.Lambda.Inputs.FunctionFileSystemConfigArgs
        {
            Arn = accessPointForLambda.Arn,
            LocalMountPath = "/mnt/efs",
        },
        VpcConfig = new Aws.Lambda.Inputs.FunctionVpcConfigArgs
        {
            SubnetIds = new[]
            {
                subnetForLambda.Id,
            },
            SecurityGroupIds = new[]
            {
                sgForLambda.Id,
            },
        },
    }, new CustomResourceOptions
    {
        DependsOn =
        {
            alpha,
        },
    });

});
```
```go
package main

import (
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/efs"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/lambda"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		// EFS file system
		efsForLambda, err := efs.NewFileSystem(ctx, "efs_for_lambda", &efs.FileSystemArgs{
			Tags: pulumi.StringMap{
				"Name": pulumi.String("efs_for_lambda"),
			},
		})
		if err != nil {
			return err
		}
		// Mount target connects the file system to the subnet
		alpha, err := efs.NewMountTarget(ctx, "alpha", &efs.MountTargetArgs{
			FileSystemId: efsForLambda.ID(),
			SubnetId:     pulumi.Any(subnetForLambda.Id),
			SecurityGroups: pulumi.StringArray{
				sgForLambda.Id,
			},
		})
		if err != nil {
			return err
		}
		// EFS access point used by lambda file system
		accessPointForLambda, err := efs.NewAccessPoint(ctx, "access_point_for_lambda", &efs.AccessPointArgs{
			FileSystemId: efsForLambda.ID(),
			RootDirectory: &efs.AccessPointRootDirectoryArgs{
				Path: pulumi.String("/lambda"),
				CreationInfo: &efs.AccessPointRootDirectoryCreationInfoArgs{
					OwnerGid:    pulumi.Int(1000),
					OwnerUid:    pulumi.Int(1000),
					Permissions: pulumi.String("777"),
				},
			},
			PosixUser: &efs.AccessPointPosixUserArgs{
				Gid: pulumi.Int(1000),
				Uid: pulumi.Int(1000),
			},
		})
		if err != nil {
			return err
		}
		// A lambda function connected to an EFS file system
		_, err = lambda.NewFunction(ctx, "example", &lambda.FunctionArgs{
			FileSystemConfig: &lambda.FunctionFileSystemConfigArgs{
				Arn:            accessPointForLambda.Arn,
				LocalMountPath: pulumi.String("/mnt/efs"),
			},
			VpcConfig: &lambda.FunctionVpcConfigArgs{
				SubnetIds: pulumi.StringArray{
					subnetForLambda.Id,
				},
				SecurityGroupIds: pulumi.StringArray{
					sgForLambda.Id,
				},
			},
		}, pulumi.DependsOn([]pulumi.Resource{
			alpha,
		}))
		if err != nil {
			return err
		}
		return nil
	})
}
```
```java
package generated_program;

import com.pulumi.Context;
import com.pulumi.Pulumi;
import com.pulumi.core.Output;
import com.pulumi.aws.efs.FileSystem;
import com.pulumi.aws.efs.FileSystemArgs;
import com.pulumi.aws.efs.MountTarget;
import com.pulumi.aws.efs.MountTargetArgs;
import com.pulumi.aws.efs.AccessPoint;
import com.pulumi.aws.efs.AccessPointArgs;
import com.pulumi.aws.efs.inputs.AccessPointRootDirectoryArgs;
import com.pulumi.aws.efs.inputs.AccessPointRootDirectoryCreationInfoArgs;
import com.pulumi.aws.efs.inputs.AccessPointPosixUserArgs;
import com.pulumi.aws.lambda.Function;
import com.pulumi.aws.lambda.FunctionArgs;
import com.pulumi.aws.lambda.inputs.FunctionFileSystemConfigArgs;
import com.pulumi.aws.lambda.inputs.FunctionVpcConfigArgs;
import com.pulumi.resources.CustomResourceOptions;
import java.util.List;
import java.util.ArrayList;
import java.util.Map;
import java.io.File;
import java.nio.file.Files;
import java.nio.file.Paths;

public class App {
    public static void main(String[] args) {
        Pulumi.run(App::stack);
    }

    public static void stack(Context ctx) {
        // EFS file system
        var efsForLambda = new FileSystem("efsForLambda", FileSystemArgs.builder()
            .tags(Map.of("Name", "efs_for_lambda"))
            .build());

        // Mount target connects the file system to the subnet
        var alpha = new MountTarget("alpha", MountTargetArgs.builder()
            .fileSystemId(efsForLambda.id())
            .subnetId(subnetForLambda.id())
            .securityGroups(sgForLambda.id())
            .build());

        // EFS access point used by lambda file system
        var accessPointForLambda = new AccessPoint("accessPointForLambda", AccessPointArgs.builder()
            .fileSystemId(efsForLambda.id())
            .rootDirectory(AccessPointRootDirectoryArgs.builder()
                .path("/lambda")
                .creationInfo(AccessPointRootDirectoryCreationInfoArgs.builder()
                    .ownerGid(1000)
                    .ownerUid(1000)
                    .permissions("777")
                    .build())
                .build())
            .posixUser(AccessPointPosixUserArgs.builder()
                .gid(1000)
                .uid(1000)
                .build())
            .build());

        // A lambda function connected to an EFS file system
        var example = new Function("example", FunctionArgs.builder()
            .fileSystemConfig(FunctionFileSystemConfigArgs.builder()
                .arn(accessPointForLambda.arn())
                .localMountPath("/mnt/efs")
                .build())
            .vpcConfig(FunctionVpcConfigArgs.builder()
                .subnetIds(subnetForLambda.id())
                .securityGroupIds(sgForLambda.id())
                .build())
            .build(), CustomResourceOptions.builder()
                .dependsOn(alpha)
                .build());

    }
}
```
```yaml
resources:
  # A lambda function connected to an EFS file system
  example:
    type: aws:lambda:Function
    properties:
      fileSystemConfig:
        arn: ${accessPointForLambda.arn}
        localMountPath: /mnt/efs
      vpcConfig:
        subnetIds:
          - ${subnetForLambda.id}
        securityGroupIds:
          - ${sgForLambda.id}
    options:
      dependson:
        - ${alpha}
  # EFS file system
  efsForLambda:
    type: aws:efs:FileSystem
    name: efs_for_lambda
    properties:
      tags:
        Name: efs_for_lambda
  # Mount target connects the file system to the subnet
  alpha:
    type: aws:efs:MountTarget
    properties:
      fileSystemId: ${efsForLambda.id}
      subnetId: ${subnetForLambda.id}
      securityGroups:
        - ${sgForLambda.id}
  # EFS access point used by lambda file system
  accessPointForLambda:
    type: aws:efs:AccessPoint
    name: access_point_for_lambda
    properties:
      fileSystemId: ${efsForLambda.id}
      rootDirectory:
        path: /lambda
        creationInfo:
          ownerGid: 1000
          ownerUid: 1000
          permissions: '777'
      posixUser:
        gid: 1000
        uid: 1000
```
<!--End PulumiCodeChooser -->

### Lambda retries

Lambda Functions allow you to configure error handling for asynchronous invocation. The settings that it supports are `Maximum age of event` and `Retry attempts` as stated in [Lambda documentation for Configuring error handling for asynchronous invocation](https://docs.aws.amazon.com/lambda/latest/dg/invocation-async.html#invocation-async-errors). To configure these settings, refer to the aws_lambda_function_event_invoke_config resource.

## CloudWatch Logging and Permissions

For more information about CloudWatch Logs for Lambda, see the [Lambda User Guide](https://docs.aws.amazon.com/lambda/latest/dg/monitoring-functions-logs.html).

<!--Start PulumiCodeChooser -->
```typescript
import * as pulumi from "@pulumi/pulumi";
import * as aws from "@pulumi/aws";

const config = new pulumi.Config();
const lambdaFunctionName = config.get("lambdaFunctionName") || "lambda_function_name";
// This is to optionally manage the CloudWatch Log Group for the Lambda Function.
// If skipping this resource configuration, also add "logs:CreateLogGroup" to the IAM policy below.
const example = new aws.cloudwatch.LogGroup("example", {
    name: `/aws/lambda/${lambdaFunctionName}`,
    retentionInDays: 14,
});
// See also the following AWS managed policy: AWSLambdaBasicExecutionRole
const lambdaLogging = aws.iam.getPolicyDocument({
    statements: [{
        effect: "Allow",
        actions: [
            "logs:CreateLogGroup",
            "logs:CreateLogStream",
            "logs:PutLogEvents",
        ],
        resources: ["arn:aws:logs:*:*:*"],
    }],
});
const lambdaLoggingPolicy = new aws.iam.Policy("lambda_logging", {
    name: "lambda_logging",
    path: "/",
    description: "IAM policy for logging from a lambda",
    policy: lambdaLogging.then(lambdaLogging => lambdaLogging.json),
});
const lambdaLogs = new aws.iam.RolePolicyAttachment("lambda_logs", {
    role: iamForLambda.name,
    policyArn: lambdaLoggingPolicy.arn,
});
const testLambda = new aws.lambda.Function("test_lambda", {
    name: lambdaFunctionName,
    loggingConfig: {
        logFormat: "Text",
    },
}, {
    dependsOn: [
        lambdaLogs,
        example,
    ],
});
```
```python
import pulumi
import pulumi_aws as aws

config = pulumi.Config()
lambda_function_name = config.get("lambdaFunctionName")
if lambda_function_name is None:
    lambda_function_name = "lambda_function_name"
# This is to optionally manage the CloudWatch Log Group for the Lambda Function.
# If skipping this resource configuration, also add "logs:CreateLogGroup" to the IAM policy below.
example = aws.cloudwatch.LogGroup("example",
    name=f"/aws/lambda/{lambda_function_name}",
    retention_in_days=14)
# See also the following AWS managed policy: AWSLambdaBasicExecutionRole
lambda_logging = aws.iam.get_policy_document(statements=[aws.iam.GetPolicyDocumentStatementArgs(
    effect="Allow",
    actions=[
        "logs:CreateLogGroup",
        "logs:CreateLogStream",
        "logs:PutLogEvents",
    ],
    resources=["arn:aws:logs:*:*:*"],
)])
lambda_logging_policy = aws.iam.Policy("lambda_logging",
    name="lambda_logging",
    path="/",
    description="IAM policy for logging from a lambda",
    policy=lambda_logging.json)
lambda_logs = aws.iam.RolePolicyAttachment("lambda_logs",
    role=iam_for_lambda["name"],
    policy_arn=lambda_logging_policy.arn)
test_lambda = aws.lambda_.Function("test_lambda",
    name=lambda_function_name,
    logging_config=aws.lambda_.FunctionLoggingConfigArgs(
        log_format="Text",
    ),
    opts=pulumi.ResourceOptions(depends_on=[
            lambda_logs,
            example,
        ]))
```
```csharp
using System.Collections.Generic;
using System.Linq;
using Pulumi;
using Aws = Pulumi.Aws;

return await Deployment.RunAsync(() => 
{
    var config = new Config();
    var lambdaFunctionName = config.Get("lambdaFunctionName") ?? "lambda_function_name";
    // This is to optionally manage the CloudWatch Log Group for the Lambda Function.
    // If skipping this resource configuration, also add "logs:CreateLogGroup" to the IAM policy below.
    var example = new Aws.CloudWatch.LogGroup("example", new()
    {
        Name = $"/aws/lambda/{lambdaFunctionName}",
        RetentionInDays = 14,
    });

    // See also the following AWS managed policy: AWSLambdaBasicExecutionRole
    var lambdaLogging = Aws.Iam.GetPolicyDocument.Invoke(new()
    {
        Statements = new[]
        {
            new Aws.Iam.Inputs.GetPolicyDocumentStatementInputArgs
            {
                Effect = "Allow",
                Actions = new[]
                {
                    "logs:CreateLogGroup",
                    "logs:CreateLogStream",
                    "logs:PutLogEvents",
                },
                Resources = new[]
                {
                    "arn:aws:logs:*:*:*",
                },
            },
        },
    });

    var lambdaLoggingPolicy = new Aws.Iam.Policy("lambda_logging", new()
    {
        Name = "lambda_logging",
        Path = "/",
        Description = "IAM policy for logging from a lambda",
        PolicyDocument = lambdaLogging.Apply(getPolicyDocumentResult => getPolicyDocumentResult.Json),
    });

    var lambdaLogs = new Aws.Iam.RolePolicyAttachment("lambda_logs", new()
    {
        Role = iamForLambda.Name,
        PolicyArn = lambdaLoggingPolicy.Arn,
    });

    var testLambda = new Aws.Lambda.Function("test_lambda", new()
    {
        Name = lambdaFunctionName,
        LoggingConfig = new Aws.Lambda.Inputs.FunctionLoggingConfigArgs
        {
            LogFormat = "Text",
        },
    }, new CustomResourceOptions
    {
        DependsOn =
        {
            lambdaLogs,
            example,
        },
    });

});
```
```go
package main

import (
	"fmt"

	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/cloudwatch"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/iam"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/lambda"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		cfg := config.New(ctx, "")
		lambdaFunctionName := "lambda_function_name"
		if param := cfg.Get("lambdaFunctionName"); param != "" {
			lambdaFunctionName = param
		}
		// This is to optionally manage the CloudWatch Log Group for the Lambda Function.
		// If skipping this resource configuration, also add "logs:CreateLogGroup" to the IAM policy below.
		example, err := cloudwatch.NewLogGroup(ctx, "example", &cloudwatch.LogGroupArgs{
			Name:            pulumi.String(fmt.Sprintf("/aws/lambda/%v", lambdaFunctionName)),
			RetentionInDays: pulumi.Int(14),
		})
		if err != nil {
			return err
		}
		// See also the following AWS managed policy: AWSLambdaBasicExecutionRole
		lambdaLogging, err := iam.GetPolicyDocument(ctx, &iam.GetPolicyDocumentArgs{
			Statements: []iam.GetPolicyDocumentStatement{
				{
					Effect: pulumi.StringRef("Allow"),
					Actions: []string{
						"logs:CreateLogGroup",
						"logs:CreateLogStream",
						"logs:PutLogEvents",
					},
					Resources: []string{
						"arn:aws:logs:*:*:*",
					},
				},
			},
		}, nil)
		if err != nil {
			return err
		}
		lambdaLoggingPolicy, err := iam.NewPolicy(ctx, "lambda_logging", &iam.PolicyArgs{
			Name:        pulumi.String("lambda_logging"),
			Path:        pulumi.String("/"),
			Description: pulumi.String("IAM policy for logging from a lambda"),
			Policy:      pulumi.String(lambdaLogging.Json),
		})
		if err != nil {
			return err
		}
		lambdaLogs, err := iam.NewRolePolicyAttachment(ctx, "lambda_logs", &iam.RolePolicyAttachmentArgs{
			Role:      pulumi.Any(iamForLambda.Name),
			PolicyArn: lambdaLoggingPolicy.Arn,
		})
		if err != nil {
			return err
		}
		_, err = lambda.NewFunction(ctx, "test_lambda", &lambda.FunctionArgs{
			Name: pulumi.String(lambdaFunctionName),
			LoggingConfig: &lambda.FunctionLoggingConfigArgs{
				LogFormat: pulumi.String("Text"),
			},
		}, pulumi.DependsOn([]pulumi.Resource{
			lambdaLogs,
			example,
		}))
		if err != nil {
			return err
		}
		return nil
	})
}
```
```java
package generated_program;

import com.pulumi.Context;
import com.pulumi.Pulumi;
import com.pulumi.core.Output;
import com.pulumi.aws.cloudwatch.LogGroup;
import com.pulumi.aws.cloudwatch.LogGroupArgs;
import com.pulumi.aws.iam.IamFunctions;
import com.pulumi.aws.iam.inputs.GetPolicyDocumentArgs;
import com.pulumi.aws.iam.Policy;
import com.pulumi.aws.iam.PolicyArgs;
import com.pulumi.aws.iam.RolePolicyAttachment;
import com.pulumi.aws.iam.RolePolicyAttachmentArgs;
import com.pulumi.aws.lambda.Function;
import com.pulumi.aws.lambda.FunctionArgs;
import com.pulumi.aws.lambda.inputs.FunctionLoggingConfigArgs;
import com.pulumi.resources.CustomResourceOptions;
import java.util.List;
import java.util.ArrayList;
import java.util.Map;
import java.io.File;
import java.nio.file.Files;
import java.nio.file.Paths;

public class App {
    public static void main(String[] args) {
        Pulumi.run(App::stack);
    }

    public static void stack(Context ctx) {
        final var config = ctx.config();
        final var lambdaFunctionName = config.get("lambdaFunctionName").orElse("lambda_function_name");
        // This is to optionally manage the CloudWatch Log Group for the Lambda Function.
        // If skipping this resource configuration, also add "logs:CreateLogGroup" to the IAM policy below.
        var example = new LogGroup("example", LogGroupArgs.builder()
            .name(String.format("/aws/lambda/%s", lambdaFunctionName))
            .retentionInDays(14)
            .build());

        // See also the following AWS managed policy: AWSLambdaBasicExecutionRole
        final var lambdaLogging = IamFunctions.getPolicyDocument(GetPolicyDocumentArgs.builder()
            .statements(GetPolicyDocumentStatementArgs.builder()
                .effect("Allow")
                .actions(                
                    "logs:CreateLogGroup",
                    "logs:CreateLogStream",
                    "logs:PutLogEvents")
                .resources("arn:aws:logs:*:*:*")
                .build())
            .build());

        var lambdaLoggingPolicy = new Policy("lambdaLoggingPolicy", PolicyArgs.builder()
            .name("lambda_logging")
            .path("/")
            .description("IAM policy for logging from a lambda")
            .policy(lambdaLogging.applyValue(getPolicyDocumentResult -> getPolicyDocumentResult.json()))
            .build());

        var lambdaLogs = new RolePolicyAttachment("lambdaLogs", RolePolicyAttachmentArgs.builder()
            .role(iamForLambda.name())
            .policyArn(lambdaLoggingPolicy.arn())
            .build());

        var testLambda = new Function("testLambda", FunctionArgs.builder()
            .name(lambdaFunctionName)
            .loggingConfig(FunctionLoggingConfigArgs.builder()
                .logFormat("Text")
                .build())
            .build(), CustomResourceOptions.builder()
                .dependsOn(                
                    lambdaLogs,
                    example)
                .build());

    }
}
```
```yaml
configuration:
  lambdaFunctionName:
    type: string
    default: lambda_function_name
resources:
  testLambda:
    type: aws:lambda:Function
    name: test_lambda
    properties:
      name: ${lambdaFunctionName}
      loggingConfig:
        logFormat: Text
    options:
      dependson:
        - ${lambdaLogs}
        - ${example}
  # This is to optionally manage the CloudWatch Log Group for the Lambda Function.
  # If skipping this resource configuration, also add "logs:CreateLogGroup" to the IAM policy below.
  example:
    type: aws:cloudwatch:LogGroup
    properties:
      name: /aws/lambda/${lambdaFunctionName}
      retentionInDays: 14
  lambdaLoggingPolicy:
    type: aws:iam:Policy
    name: lambda_logging
    properties:
      name: lambda_logging
      path: /
      description: IAM policy for logging from a lambda
      policy: ${lambdaLogging.json}
  lambdaLogs:
    type: aws:iam:RolePolicyAttachment
    name: lambda_logs
    properties:
      role: ${iamForLambda.name}
      policyArn: ${lambdaLoggingPolicy.arn}
variables:
  # See also the following AWS managed policy: AWSLambdaBasicExecutionRole
  lambdaLogging:
    fn::invoke:
      Function: aws:iam:getPolicyDocument
      Arguments:
        statements:
          - effect: Allow
            actions:
              - logs:CreateLogGroup
              - logs:CreateLogStream
              - logs:PutLogEvents
            resources:
              - arn:aws:logs:*:*:*
```
<!--End PulumiCodeChooser -->

## Specifying the Deployment Package

AWS Lambda expects source code to be provided as a deployment package whose structure varies depending on which `runtime` is in use. See [Runtimes](https://docs.aws.amazon.com/lambda/latest/dg/API_CreateFunction.html#SSS-CreateFunction-request-Runtime) for the valid values of `runtime`. The expected structure of the deployment package can be found in [the AWS Lambda documentation for each runtime](https://docs.aws.amazon.com/lambda/latest/dg/deployment-package-v2.html).

Once you have created your deployment package you can specify it either directly as a local file (using the `filename` argument) or indirectly via Amazon S3 (using the `s3_bucket`, `s3_key` and `s3_object_version` arguments). When providing the deployment package via S3 it may be useful to use the `aws_s3_object` resource to upload it.

For larger deployment packages it is recommended by Amazon to upload via S3, since the S3 API has better support for uploading large files efficiently.

## Import

Using `pulumi import`, import Lambda Functions using the `function_name`. For example:

```sh
$ pulumi import aws:lambda/function:Function test_lambda my_test_lambda_function
```
