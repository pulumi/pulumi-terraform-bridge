Provides a Lambda Function resource. Lambda allows you to trigger execution of code in response to events in AWS, enabling serverless backend solutions. The Lambda Function itself includes source code and runtime configuration.

For information about Lambda and how to use it, see [What is AWS Lambda?][1]

For a detailed example of setting up Lambda and API Gateway, see [Serverless Applications with AWS Lambda and API Gateway.][11]

~> **NOTE:** Due to [AWS Lambda improved VPC networking changes that began deploying in September 2019](https://aws.amazon.com/blogs/compute/announcing-improved-vpc-networking-for-aws-lambda-functions/), EC2 subnets and security groups associated with Lambda Functions can take up to 45 minutes to successfully delete.

~> **NOTE:** If you get a `KMSAccessDeniedException: Lambda was unable to decrypt the environment variables because KMS access was denied` error when invoking an `aws_lambda_function` with environment variables, the IAM role associated with the function may have been deleted and recreated _after_ the function was created. You can fix the problem two ways: 1) updating the function's role to another role and then updating it back again to the recreated role, or 2) by using Terraform to `taint` the function and `apply` your configuration again to recreate the function. (When you create a function, Lambda grants permissions on the KMS key to the function's IAM role. If the IAM role is recreated, the grant is no longer valid. Changing the function's role or recreating the function causes Lambda to update the grant.)

-> To give an external source (like an EventBridge Rule, SNS, or S3) permission to access the Lambda function, use the `aws_lambda_permission` resource. See [Lambda Permission Model][4] for more details. On the other hand, the `role` argument of this resource is the function's execution role for identity and access to AWS services and resources.

## Example Usage

### Basic Example

```terraform
data "aws_iam_policy_document" "assume_role" {
  statement {
    effect = "Allow"

    principals {
      type        = "Service"
      identifiers = ["lambda.amazonaws.com"]
    }

    actions = ["sts:AssumeRole"]
  }
}

resource "aws_iam_role" "iam_for_lambda" {
  name               = "iam_for_lambda"
  assume_role_policy = data.aws_iam_policy_document.assume_role.json
}

data "archive_file" "lambda" {
  type        = "zip"
  source_file = "lambda.js"
  output_path = "lambda_function_payload.zip"
}

resource "aws_lambda_function" "test_lambda" {
  # If the file is not in the current working directory you will need to include a
  # path.module in the filename.
  filename      = "lambda_function_payload.zip"
  function_name = "lambda_function_name"
  role          = aws_iam_role.iam_for_lambda.arn
  handler       = "index.test"

  source_code_hash = data.archive_file.lambda.output_base64sha256

  runtime = "nodejs18.x"

  environment {
    variables = {
      foo = "bar"
    }
  }
}
```

### Lambda Layers

```terraform
resource "aws_lambda_layer_version" "example" {
  # ... other configuration ...
}

resource "aws_lambda_function" "example" {
  # ... other configuration ...
  layers = [aws_lambda_layer_version.example.arn]
}
```

### Lambda Ephemeral Storage

Lambda Function Ephemeral Storage(`/tmp`) allows you to configure the storage upto `10` GB. The default value set to `512` MB.

```terraform
data "aws_iam_policy_document" "assume_role" {
  statement {
    effect = "Allow"

    principals {
      type        = "Service"
      identifiers = ["lambda.amazonaws.com"]
    }

    actions = ["sts:AssumeRole"]
  }
}

resource "aws_iam_role" "iam_for_lambda" {
  name               = "iam_for_lambda"
  assume_role_policy = data.aws_iam_policy_document.assume_role.json
}

resource "aws_lambda_function" "test_lambda" {
  filename      = "lambda_function_payload.zip"
  function_name = "lambda_function_name"
  role          = aws_iam_role.iam_for_lambda.arn
  handler       = "index.test"
  runtime       = "nodejs18.x"

  ephemeral_storage {
    size = 10240 # Min 512 MB and the Max 10240 MB
  }
}
```

### Lambda File Systems

Lambda File Systems allow you to connect an Amazon Elastic File System (EFS) file system to a Lambda function to share data across function invocations, access existing data including large files, and save function state.

```terraform
# A lambda function connected to an EFS file system
resource "aws_lambda_function" "example" {
  # ... other configuration ...

  file_system_config {
    # EFS file system access point ARN
    arn = aws_efs_access_point.access_point_for_lambda.arn

    # Local mount path inside the lambda function. Must start with '/mnt/'.
    local_mount_path = "/mnt/efs"
  }

  vpc_config {
    # Every subnet should be able to reach an EFS mount target in the same Availability Zone. Cross-AZ mounts are not permitted.
    subnet_ids         = [aws_subnet.subnet_for_lambda.id]
    security_group_ids = [aws_security_group.sg_for_lambda.id]
  }

  # Explicitly declare dependency on EFS mount target.
  # When creating or updating Lambda functions, mount target must be in 'available' lifecycle state.
  depends_on = [aws_efs_mount_target.alpha]
}

# EFS file system
resource "aws_efs_file_system" "efs_for_lambda" {
  tags = {
    Name = "efs_for_lambda"
  }
}

# Mount target connects the file system to the subnet
resource "aws_efs_mount_target" "alpha" {
  file_system_id  = aws_efs_file_system.efs_for_lambda.id
  subnet_id       = aws_subnet.subnet_for_lambda.id
  security_groups = [aws_security_group.sg_for_lambda.id]
}

# EFS access point used by lambda file system
resource "aws_efs_access_point" "access_point_for_lambda" {
  file_system_id = aws_efs_file_system.efs_for_lambda.id

  root_directory {
    path = "/lambda"
    creation_info {
      owner_gid   = 1000
      owner_uid   = 1000
      permissions = "777"
    }
  }

  posix_user {
    gid = 1000
    uid = 1000
  }
}
```

### Lambda retries

Lambda Functions allow you to configure error handling for asynchronous invocation. The settings that it supports are `Maximum age of event` and `Retry attempts` as stated in [Lambda documentation for Configuring error handling for asynchronous invocation](https://docs.aws.amazon.com/lambda/latest/dg/invocation-async.html#invocation-async-errors). To configure these settings, refer to the aws_lambda_function_event_invoke_config resource.

## CloudWatch Logging and Permissions

For more information about CloudWatch Logs for Lambda, see the [Lambda User Guide](https://docs.aws.amazon.com/lambda/latest/dg/monitoring-functions-logs.html).

```terraform
variable "lambda_function_name" {
  default = "lambda_function_name"
}

resource "aws_lambda_function" "test_lambda" {
  function_name = var.lambda_function_name

  # Advanced logging controls (optional)
  logging_config {
    log_format = "Text"
  }

  # ... other configuration ...
  depends_on = [
    aws_iam_role_policy_attachment.lambda_logs,
    aws_cloudwatch_log_group.example,
  ]
}

# This is to optionally manage the CloudWatch Log Group for the Lambda Function.
# If skipping this resource configuration, also add "logs:CreateLogGroup" to the IAM policy below.
resource "aws_cloudwatch_log_group" "example" {
  name              = "/aws/lambda/${var.lambda_function_name}"
  retention_in_days = 14
}

# See also the following AWS managed policy: AWSLambdaBasicExecutionRole
data "aws_iam_policy_document" "lambda_logging" {
  statement {
    effect = "Allow"

    actions = [
      "logs:CreateLogGroup",
      "logs:CreateLogStream",
      "logs:PutLogEvents",
    ]

    resources = ["arn:aws:logs:*:*:*"]
  }
}

resource "aws_iam_policy" "lambda_logging" {
  name        = "lambda_logging"
  path        = "/"
  description = "IAM policy for logging from a lambda"
  policy      = data.aws_iam_policy_document.lambda_logging.json
}

resource "aws_iam_role_policy_attachment" "lambda_logs" {
  role       = aws_iam_role.iam_for_lambda.name
  policy_arn = aws_iam_policy.lambda_logging.arn
}
```

## Specifying the Deployment Package

AWS Lambda expects source code to be provided as a deployment package whose structure varies depending on which `runtime` is in use. See [Runtimes][6] for the valid values of `runtime`. The expected structure of the deployment package can be found in [the AWS Lambda documentation for each runtime][8].

Once you have created your deployment package you can specify it either directly as a local file (using the `filename` argument) or indirectly via Amazon S3 (using the `s3_bucket`, `s3_key` and `s3_object_version` arguments). When providing the deployment package via S3 it may be useful to use the `aws_s3_object` resource to upload it.

For larger deployment packages it is recommended by Amazon to upload via S3, since the S3 API has better support for uploading large files efficiently.

## Import

In Terraform v1.5.0 and later, use an `import` block to import Lambda Functions using the `function_name`. For example:

```terraform
import {
  to = aws_lambda_function.test_lambda
  id = "my_test_lambda_function"
}
```

Using `terraform import`, import Lambda Functions using the `function_name`. For example:

```console
% terraform import aws_lambda_function.test_lambda my_test_lambda_function
```
