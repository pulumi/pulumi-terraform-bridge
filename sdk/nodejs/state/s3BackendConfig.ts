// Copyright 2016-2019, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

import * as pulumi from "@pulumi/pulumi";

/**
 * The configuration options for a Terraform Remote State stored in the S3 backend.
 */
export interface S3RemoteStateReferenceArgs {
    /**
     * A constant describing the name of the Terraform backend, used as the discriminant
     * for the union of backend configurations.
     */
    readonly backendType: "s3";

    /**
     * The name of the S3 bucket.
     */
    readonly bucket: pulumi.Input<string>;

    /**
     * The path to the state file inside the bucket. When using a non-default workspace,
     * the state path will be `/workspace_key_prefix/workspace_name/key`.
     */
    readonly key: pulumi.Input<string>;

    /**
     * The region of the S3 bucket. Also sourced from `AWS_DEFAULT_REGION` in the environment, if unset.
     */
    readonly region?: pulumi.Input<string>;

    /**
     * A custom endpoint for the S3 API. Also sourced from `AWS_S3_ENDPOINT` in the environment, if unset.
     */
    readonly endpoint?: pulumi.Input<string>;

    /**
     * AWS Access Key. Sourced from the standard credentials pipeline, if unset.
     */
    readonly accessKey?: pulumi.Input<string>;

    /**
     * AWS Secret Access Key. Sourced from the standard credentials pipeline, if unset.
     */
    readonly secretKey?: pulumi.Input<string>;

    /**
     * The AWS profile name as set in the shared credentials file.
     */
    readonly profile?: pulumi.Input<string>;

    /**
     * The path to the shared credentials file. If this is not set and a profile is
     * specified, `~/.aws/credentials` will be used by default.
     */
    readonly sharedCredentialsFile?: pulumi.Input<string>;

    /**
     * An MFA token. Sourced from the `AWS_SESSION_TOKEN` in the environment variable if needed and unset.
     */
    readonly token?: pulumi.Input<string>;

    /**
     * The ARN of an IAM Role to be assumed in order to read the state from S3.
     */
    readonly roleArn?: pulumi.Input<string>;

    /**
     * The external ID to use when assuming the IAM role.
     */
    readonly externalId?: pulumi.Input<string>;

    /**
     * The session name to use when assuming the IAM role.
     */
    readonly sessionName?: pulumi.Input<string>;

    /**
     * The prefix applied to the state path inside the bucket. This is only relevant when
     * using a non-default workspace, and defaults to `env:`.
     */
    readonly workspaceKeyPrefix?: pulumi.Input<string>;

    /**
     * A custom endpoint for the IAM API. Sourced from `AWS_IAM_ENDPOINT`, if unset.
     */
    readonly iamEndpoint?: pulumi.Input<string>;

    /**
     * A custom endpoint for the STS API. Sourced from `AWS_STS_ENDPOINT`, if unset.
     */
    readonly stsEndpoint?: pulumi.Input<string>;

    /**
     * The Terraform workspace from which to read state.
     */
    readonly workspace?: pulumi.Input<string>;
}

