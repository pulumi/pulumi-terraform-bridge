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
 * The configuration options for a Terraform Remote State stored in the Google Cloud Storage
 * backend.
 */
export interface GCSRemoteStateReferenceArgs {
    /**
     * A constant describing the name of the Terraform backend, used as the discriminant
     * for the union of backend configurations.
     */
    backendType: "gcs";

    /**
     * The name of the Google Cloud Storage bucket.
     */
    bucket: pulumi.Input<string>;

    /**
     * Local path to Google Cloud Platform account credentials in JSON format. Sourced from
     * `GOOGLE_CREDENTIALS` in the environment if unset. If no value is provided Google
     * Application Default Credentials are used.
     */
    credentials?: pulumi.Input<string>;

    /**
     * Prefix used inside the Google Cloud Storage bucket. Named states for workspaces
     * are stored in an object named `<prefix>/<name>.tfstate`.
     */
    prefix?: pulumi.Input<string>;

    /**
     * A 32 byte, base64-encoded customer supplied encryption key used to encrypt the
     * state. Sourced from `GOOGLE_ENCRYPTION_KEY` in the environment, if unset.
     */
    encryptionKey?: pulumi.Input<string>;

    /**
     * The Terraform workspace from which to read state.
     */
    workspace?: pulumi.Input<string>;
}
