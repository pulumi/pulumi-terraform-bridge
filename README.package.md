# Pulumi Terraform Provider

The Terraform resource provider for Pulumi lets you consume the outputs
contained in Terraform state files from your Pulumi programs. The package
provides a `RemoteStateReference` resource which acts like a native Pulumi
[`StackReference`][stackreference].

To use this package, please [install the Pulumi CLI first][pulumicli].

## Installing

Currently, the Terraform Provider is available only for Node.js, and is
distributed as an `npm` package.

### Node.js (JavaScript/TypeScript)

To use from JavaScript or TypeScript in Node.js, install using either `npm`:

    $ npm install @pulumi/terraform

or `yarn`:

    $ yarn add @pulumi/terraform

## Concepts

The `@pulumi/terraform` package provides a resource named `RemoteStateReference`
which is used to read outputs from a Terraform state file stored in one of the
supported Terraform remote state backends.

## Examples

### S3

The following program will read a Terraform state file stored in S3:

```typescript
import * as tf from "@pulumi/terraform";

const remoteState = new tf.state.RemoteStateReference("s3state", {
    backendType: "s3",
    bucket: "pulumi-terraform-state-test",
    key: "test/terraform.tfstate",
    region: "us-west-2"
});

// Use the getOutput function on the resource to access root outputs
const vpcId= remoteState.getOutput("vpc_id");
```

### Local file

The following program will read a Terraform state file stored locally in the
filesystem:

```typescript
import * as tf from "@pulumi/terraform";

const remotestate = new tf.state.RemoteStateReference("localstate", {
   backendType: "local",
   path: path.join(__dirname, "terraform.tfstate"),
});

// Use the getOutput function on the resource to access root outputs
const vpcId= remoteState.getOutput("vpc_id");
```

### Terraform Enterprise

For state stored in Terraform Enterprise, the authentication token must be set
via the Pulumi configuration system - for example, using:

    pulumi config set --secret terraformEnterpriseToken <value>

The following program will read a Terraform state file stored in Terraform
Enterprise, using the value of `terraformEnterpriseToken` from above:

```typescript
import * as pulumi from "@pulumi/pulumi";
import * as tf from "@pulumi/terraform";

const config = new pulumi.Config();

const ref = new tf.state.RemoteStateReference("remote", {
    backendType: "remote",
    organization: "pulumi",
    token: config.requireSecret("terraformEnterpriseToken"),
    workspaces: {
        name: "test-state-file"
    }
});

// Use the getOutput function on the resource to access root outputs
const vpcId= remoteState.getOutput("vpc_id");
```

[stackreference]: https://www.pulumi.com/docs/reference/organizing-stacks-projects/#inter-stack-dependencies
[pulumicli]: https://pulumi.com/

